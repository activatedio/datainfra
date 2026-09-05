package testing

import (
	"context"
	"testing"

	"go.uber.org/fx"
)

// Tolerance is what a test declares about the state of the database it is
// handed.
type Tolerance int

const (
	// Tolerant tests accept rows left behind by other tests, as long as the
	// layers they asked for are applied. They may write. They share the
	// database with other tolerant tests running at the same time.
	Tolerant Tolerance = iota

	// Pristine tests need the database exactly as freshly migrated: the
	// layers they asked for applied and nothing else written since. They
	// hold the database exclusively while they run.
	Pristine
)

func (t Tolerance) String() string {
	switch t {
	case Tolerant:
		return "tolerant"
	case Pristine:
		return "pristine"
	default:
		return "unknown"
	}
}

// Requirement is what a test asks of its fixture.
type Requirement struct {
	// Stack names the layers the test needs, in the fixture's declared
	// order. Nil means the whole stack.
	Stack []string

	// Tolerance is how clean the database must be. The zero value is
	// Tolerant.
	Tolerance Tolerance

	// Dedicated asks for a database of the test's own, dropped when the test
	// ends, instead of the profile's shared one.
	Dedicated bool
}

// AppFixtureResult represents the result of setting up a test fixture, including the app instance and its name.
type AppFixtureResult struct {

	// Name is the name of the fixture
	Name string

	// App is the application option, usually an fx.Module
	App fx.Option
}

// AppFixture is what Run consumes: a fixture already bound to a Requirement.
type AppFixture interface {

	// GetApp configures and retrieves an application fixture for testing based on provided dependencies and invocation.
	GetApp(t *testing.T, toProvide ...any) AppFixtureResult

	// Cleanup drops whatever the fixture still owns at suite end. Per-test
	// work never waits for it — it hangs off t.Cleanup — so for a dedicated
	// fixture this is a no-op by the time it is called.
	Cleanup() error
}

// LifecycleFixture is what a backend implements: the same as AppFixture, with
// the Requirement chosen per call. AppFixtureRegistry binds a Requirement to
// produce the AppFixture that Run consumes.
type LifecycleFixture interface {
	GetApp(t *testing.T, req Requirement, toProvide ...any) AppFixtureResult
	Cleanup() error
}

// Bind fixes a Requirement onto a LifecycleFixture.
func Bind(f LifecycleFixture, req Requirement) AppFixture {
	return &bound{f: f, req: req}
}

type bound struct {
	f   LifecycleFixture
	req Requirement
}

func (b *bound) GetApp(t *testing.T, toProvide ...any) AppFixtureResult {
	return b.f.GetApp(t, b.req, toProvide...)
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
