package testing

import (
	"context"
	"testing"

	"go.uber.org/fx"
)

// Mode selects how a fixture's database is provisioned around one test.
//
// Every fixture runs the same four rings — Setup, MigrateUp, the test,
// MigrateDown, Teardown — and Mode decides which rings belong to this test
// and which to the shared database the test borrows:
//
//	Mode               Setup      MigrateUp        MigrateDown      Teardown
//	Reuse              shared     base+delta once  never            suite end
//	ReuseWithMigrate   shared     base once, delta per test, delta undone after
//	Fresh              this test  base+delta       skipped (drop)   this test
//
// "base" is the graph's untagged migrate.Migrator — schema, and data the
// database carries for life. "delta" is the graph's migrate.Reversible tagged
// name:"delta" — what one test needs on top, and must remove afterwards.
type Mode int

const (
	// ModeReuse borrows the profile's shared database as-is. Base and delta
	// are applied once, by whichever test arrives first, and never reverted;
	// the database is dropped when the suite ends. Tests in this mode must
	// tolerate each other's rows.
	ModeReuse Mode = iota

	// ModeReuseWithMigrate borrows the shared database but applies the delta
	// for this test alone and reverts it when the test ends. Deltas on one
	// database are serialized. The base must be schema-only when the delta's
	// Down is a table reset.
	ModeReuseWithMigrate

	// ModeFresh provisions a database for this test alone — base and delta
	// applied — and drops it when the test ends. Nothing is reverted; the
	// drop is the undo.
	ModeFresh
)

func (m Mode) String() string {
	switch m {
	case ModeReuse:
		return "reuse"
	case ModeReuseWithMigrate:
		return "reuse-with-migrate"
	case ModeFresh:
		return "fresh"
	default:
		return "unknown"
	}
}

// AppFixtureResult represents the result of setting up a test fixture, including the app instance and its name.
type AppFixtureResult struct {

	// Name is the name of the fixture
	Name string

	// App is the application option, usually an fx.Module
	App fx.Option
}

// AppFixture is what Run consumes: a fixture already bound to a Mode.
type AppFixture interface {

	// GetApp configures and retrieves an application fixture for testing based on provided dependencies and invocation.
	GetApp(t *testing.T, toProvide ...any) AppFixtureResult

	// Cleanup drops whatever the fixture still owns at suite end. Per-test
	// rings do not wait for it — they hang off t.Cleanup — so for a fixture
	// only ever used in ModeFresh this is a no-op.
	Cleanup() error
}

// LifecycleFixture is what a backend implements: the same as AppFixture, with
// the Mode chosen per call. AppFixtureRegistry binds a Mode to produce the
// AppFixture that Run consumes.
type LifecycleFixture interface {
	GetApp(t *testing.T, mode Mode, toProvide ...any) AppFixtureResult
	Cleanup() error
}

// Bind fixes a Mode onto a LifecycleFixture.
func Bind(f LifecycleFixture, mode Mode) AppFixture {
	return &bound{f: f, mode: mode}
}

type bound struct {
	f    LifecycleFixture
	mode Mode
}

func (b *bound) GetApp(t *testing.T, toProvide ...any) AppFixtureResult {
	return b.f.GetApp(t, b.mode, toProvide...)
}

func (b *bound) Cleanup() error {
	return b.f.Cleanup()
}

// ContextProvider is an interface that provides a method to retrieve a context.Context instance.
type ContextProvider interface {

	// GetContext returns a context.Context instance.
	GetContext() context.Context

	// AfterTest closes the database connection.
	AfterTest() error
}
