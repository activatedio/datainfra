package gorm_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"gorm.io/gorm"

	bringupgorm "github.com/activatedio/datainfra/pkg/bringup/gorm"
	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/migrate"
	"github.com/activatedio/datainfra/pkg/setup"
)

// recorder captures the order things happened in; the assertions are all
// about order.
type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

// fakeSetup records that it ran, optionally failing or stalling. Stalling
// respects ctx, which is what the timeout test exercises.
type fakeSetup struct {
	rec   *recorder
	err   error
	delay time.Duration
}

func (f *fakeSetup) Setup(ctx context.Context, _ setup.Params) error {
	if f.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.delay):
		}
	}
	f.rec.add("setup")
	return f.err
}

func (f *fakeSetup) Teardown(_ context.Context) error {
	return nil
}

// fakeMigrator records that it ran, optionally failing.
type fakeMigrator struct {
	rec *recorder
	err error
}

func (f *fakeMigrator) Migrate(_ context.Context) error {
	f.rec.add("migrate")
	return f.err
}

// sqliteConfig gives the module a real dialable database with no external
// service: what is under test is ordering, not a dialect.
func sqliteConfig(t *testing.T) func() *datagorm.Config {
	t.Helper()
	config := &datagorm.Config{
		Dialect: datagorm.DialectSqlite,
		Name:    filepath.Join(t.TempDir(), "bringup.sqlite"),
	}
	return func() *datagorm.Config { return config }
}

// app assembles the module plus whatever the test provides, with an invoke
// that records when the pool became available.
func app(t *testing.T, options bringupgorm.Options, rec *recorder, provides ...any) (*fx.App, *sql.DB) {
	t.Helper()

	var sqlDB *sql.DB

	fxApp := fx.New(
		bringupgorm.Module(options),
		fx.Provide(provides...),
		fx.Invoke(func(_ *gorm.DB) {
			rec.add("pool")
		}),
		fx.Populate(&sqlDB),
		fx.NopLogger,
	)

	return fxApp, sqlDB
}

// The self-provisioning posture: setup, then migrations, then — and only
// then — the pool.
func TestStagesRunInOrderBeforeThePool(t *testing.T) {

	rec := &recorder{}

	fxApp, sqlDB := app(t,
		bringupgorm.Options{Setup: true, Migrate: true},
		rec,
		func() setup.Setup { return &fakeSetup{rec: rec} },
		func() migrate.Migrator { return &fakeMigrator{rec: rec} },
		sqliteConfig(t),
	)
	require.NoError(t, fxApp.Err())

	assert.Equal(t, []string{"setup", "migrate", "pool"}, rec.all())
	require.NotNil(t, sqlDB)
	assert.NoError(t, sqlDB.PingContext(t.Context()), "the unwrapped pool is the same, working pool")
}

// The pre-provisioned posture: no stages in the graph at all, pool gated on
// a no-op Ready. This is what a server whose migrations run in an operator
// command looks like.
func TestZeroOptionsNeedNoStages(t *testing.T) {

	rec := &recorder{}

	fxApp, sqlDB := app(t, bringupgorm.Options{}, rec, sqliteConfig(t))
	require.NoError(t, fxApp.Err())

	assert.Equal(t, []string{"pool"}, rec.all())
	assert.NoError(t, sqlDB.PingContext(t.Context()))
}

// Stages that are present but not requested are ignored — a migrate
// command's providers can share a graph with a server that must not
// re-migrate at boot.
func TestPresentButUnrequestedStagesDoNotRun(t *testing.T) {

	rec := &recorder{}

	fxApp, _ := app(t,
		bringupgorm.Options{},
		rec,
		func() setup.Setup { return &fakeSetup{rec: rec} },
		func() migrate.Migrator { return &fakeMigrator{rec: rec} },
		sqliteConfig(t),
	)
	require.NoError(t, fxApp.Err())

	assert.Equal(t, []string{"pool"}, rec.all())
}

// Requesting a stage that is not in the graph is a construction error, not a
// silent skip: a deployment that believes it self-provisions must not boot
// against a database nothing prepared.
func TestRequestedStageMustBeInTheGraph(t *testing.T) {

	rec := &recorder{}

	fxApp, _ := app(t, bringupgorm.Options{Setup: true}, rec, sqliteConfig(t))

	require.Error(t, fxApp.Err())
	assert.Contains(t, fxApp.Err().Error(), "no setup.Setup is in the graph")
	assert.Empty(t, rec.all(), "nothing runs, and no pool opens")
}

// A failed stage blocks the pool. This is the whole reason the marker
// exists: a failed migration must be a boot failure, not a server that came
// up and fails on first use.
func TestFailedMigrationBlocksThePool(t *testing.T) {

	rec := &recorder{}

	fxApp, _ := app(t,
		bringupgorm.Options{Setup: true, Migrate: true},
		rec,
		func() setup.Setup { return &fakeSetup{rec: rec} },
		func() migrate.Migrator { return &fakeMigrator{rec: rec, err: assert.AnError} },
		sqliteConfig(t),
	)

	require.Error(t, fxApp.Err())
	assert.Contains(t, fxApp.Err().Error(), "bringup: migrate")
	assert.Equal(t, []string{"setup", "migrate"}, rec.all(),
		"the stages ran and failed; the pool never opened")
}

// The timeout turns a wedged database into a bounded boot failure.
func TestTimeoutBoundsBringup(t *testing.T) {

	rec := &recorder{}

	fxApp, _ := app(t,
		bringupgorm.Options{Setup: true, Timeout: 20 * time.Millisecond},
		rec,
		func() setup.Setup { return &fakeSetup{rec: rec, delay: time.Second} },
		sqliteConfig(t),
	)

	require.Error(t, fxApp.Err())
	assert.Contains(t, fxApp.Err().Error(), "context deadline exceeded")
	assert.Empty(t, rec.all())
}
