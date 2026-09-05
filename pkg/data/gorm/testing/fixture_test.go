package testing_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"gorm.io/gorm"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	gormtesting "github.com/activatedio/datainfra/pkg/data/gorm/testing"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/activatedio/datainfra/pkg/migrate"
	gormmigrate "github.com/activatedio/datainfra/pkg/migrate/gorm"
	gormsetup "github.com/activatedio/datainfra/pkg/setup/gorm"
)

// The lifecycle is exercised on sqlite so it runs anywhere. The rings are
// dialect-independent: Setup/Teardown dispatch inside setup/gorm, which has
// its own per-dialect tests.

// recordingDelta counts Up/Down calls and writes a marker row so a test can
// see whether the delta is present in the database.
type recordingDelta struct {
	mu   sync.Mutex
	ups  int
	down int
	db   func() *gorm.DB
}

func (d *recordingDelta) Up(_ context.Context) error {
	d.mu.Lock()
	d.ups++
	d.mu.Unlock()
	return d.db().Exec(`INSERT INTO marker (id) VALUES (1)`).Error
}

func (d *recordingDelta) Down(_ context.Context) error {
	d.mu.Lock()
	d.down++
	d.mu.Unlock()
	return d.db().Exec(`DELETE FROM marker`).Error
}

func (d *recordingDelta) counts() (int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ups, d.down
}

type baseSchema struct{ db func() *gorm.DB }

func (b baseSchema) Migrate(_ context.Context) error {
	return b.db().Exec(`CREATE TABLE IF NOT EXISTS marker (id INTEGER PRIMARY KEY)`).Error
}

// newSqliteFixture builds a LifecycleFixture over a sqlite file at path with
// a one-table base and the given delta. Base and delta open their own short
// pools, as real migrators do.
func newSqliteFixture(t *testing.T, path string, delta *recordingDelta) (datatesting.LifecycleFixture, func() *gorm.DB) {
	cfg := &datagorm.Config{Dialect: datagorm.DialectSqlite, Name: path}
	open := func() *gorm.DB {
		db, err := datagorm.NewDB(cfg)
		require.NoError(t, err)
		return db
	}
	if delta != nil {
		delta.db = open
	}
	opt := fx.Module("sqlite-fixture",
		fx.Provide(
			func() *datagorm.Config { return cfg },
			func() *gormsetup.OwnerGormConfig { return &gormsetup.OwnerGormConfig{Config: *cfg} },
			func() *gormmigrate.MigratorGormConfig { return &gormmigrate.MigratorGormConfig{GormConfig: *cfg} },
			func() []gormmigrate.MigratorData { return nil },
			datagorm.NewContextBuilder,
		),
	)
	if delta != nil {
		opt = fx.Options(opt, fx.Provide(fx.Annotate(
			func() migrate.Reversible { return delta },
			fx.ResultTags(`name:"delta"`),
		)))
	}
	// gormmigrate.NewMigrator is provided by the fixture itself and would
	// clash with the base above, so the fixture is told to use ours by
	// decorating the migrator it builds.
	return gormtesting.NewAppFixture("sqlite", fx.Options(opt,
		fx.Decorate(func(_ migrate.Migrator) migrate.Migrator { return baseSchema{db: open} }),
	)), open
}

func markerCount(t *testing.T, open func() *gorm.DB) int64 {
	db := open()
	defer func() {
		sdb, _ := db.DB()
		_ = sdb.Close()
	}()
	var n int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM marker`).Scan(&n).Error)
	return n
}

func runApp(t *testing.T, res datatesting.AppFixtureResult) {
	app := fxtest.New(t, fx.NopLogger, res.App)
	app.RequireStart()
	app.RequireStop()
}

func TestLifecycleModes(t *testing.T) {

	type setup struct {
		mode  datatesting.Mode
		runs  int
		delta bool
	}

	cases := map[string]struct {
		arrange func() setup
		assert  func(t *testing.T, path string, open func() *gorm.DB, d *recordingDelta)
	}{
		"reuse applies the delta once and keeps it": {
			arrange: func() setup { return setup{mode: datatesting.ModeReuse, runs: 3, delta: true} },
			assert: func(t *testing.T, path string, open func() *gorm.DB, d *recordingDelta) {
				ups, downs := d.counts()
				require.Equal(t, 1, ups)
				require.Equal(t, 0, downs)
				require.EqualValues(t, 1, markerCount(t, open))
				_, err := os.Stat(path)
				require.NoError(t, err, "shared database survives until Cleanup")
			},
		},
		"reuse-with-migrate applies and reverts per test": {
			arrange: func() setup { return setup{mode: datatesting.ModeReuseWithMigrate, runs: 3, delta: true} },
			assert: func(t *testing.T, path string, open func() *gorm.DB, d *recordingDelta) {
				ups, downs := d.counts()
				require.Equal(t, 3, ups)
				require.Equal(t, 3, downs)
				require.EqualValues(t, 0, markerCount(t, open), "delta rows gone after each test")
				_, err := os.Stat(path)
				require.NoError(t, err, "shared database survives until Cleanup")
			},
		},
		"fresh drops the database when the test ends": {
			arrange: func() setup { return setup{mode: datatesting.ModeFresh, runs: 1, delta: true} },
			assert: func(t *testing.T, path string, _ func() *gorm.DB, d *recordingDelta) {
				ups, downs := d.counts()
				require.Equal(t, 1, ups)
				require.Equal(t, 0, downs, "the drop is the undo")
				_, err := os.Stat(path)
				require.ErrorIs(t, err, os.ErrNotExist)
			},
		},
		"no delta in the graph is fine": {
			arrange: func() setup { return setup{mode: datatesting.ModeReuseWithMigrate, runs: 2, delta: false} },
			assert: func(t *testing.T, _ string, open func() *gorm.DB, _ *recordingDelta) {
				require.EqualValues(t, 0, markerCount(t, open))
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			s := v.arrange()
			path := filepath.Join(t.TempDir(), "lifecycle.db")
			var d *recordingDelta
			if s.delta {
				d = &recordingDelta{}
			}
			fix, open := newSqliteFixture(t, path, d)

			// Each "test" is a child so its t.Cleanup — where the per-test
			// rings run — fires before the parent asserts.
			for i := 0; i < s.runs; i++ {
				t.Run("run", func(t *testing.T) {
					runApp(t, fix.GetApp(t, s.mode))
					if s.delta && s.mode != datatesting.ModeFresh {
						require.EqualValues(t, 1, markerCount(t, open), "delta present while the test runs")
					}
				})
			}

			v.assert(t, path, open, d)
			require.NoError(t, fix.Cleanup())
			_, err := os.Stat(path)
			require.ErrorIs(t, err, os.ErrNotExist, "Cleanup drops a shared database and is a no-op on a dropped one")
		})
	}
}
