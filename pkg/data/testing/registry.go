package testing

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var globalSuffixCounter atomic.Int64

// AppFixtureLifecycle manages the lifecycle of application test fixtures, supporting shared and fresh retrieval strategies.
type AppFixtureLifecycle struct {
	once    *sync.Once
	shared  AppFixture
	factory func(suffix string) AppFixture
}

// NewAppFixtureLifecycle creates a new AppFixtureLifecycle instance using the provided factory to produce fixtures on demand.
func NewAppFixtureLifecycle(factory func(suffix string) AppFixture) *AppFixtureLifecycle {
	return &AppFixtureLifecycle{
		factory: factory,
		once:    &sync.Once{},
	}
}

// Closer defines a function type responsible for releasing resources or performing cleanup operations, returning an error if any.
type Closer func() error

// GetShared retrieves or initializes a shared AppFixture instance that is reused across calls, returning a Closer only on first creation.
func (a *AppFixtureLifecycle) GetShared() (AppFixture, Closer) {

	var cl Closer
	a.once.Do(func() {
		// Only return a closer if we created the shared instance on this call
		a.shared, cl = a.GetFresh()
	})

	return a.shared, cl
}

// GetFresh creates a new AppFixture instance with a unique suffix and returns the fixture and its cleanup function.
func (a *AppFixtureLifecycle) GetFresh() (AppFixture, Closer) {
	f := a.factory(a.makeSuffix())
	return f, f.Cleanup
}

// makeSuffix generates a unique suffix string based on the current timestamp in milliseconds, the process ID,
// and a monotonically increasing counter to prevent collisions when multiple fixtures are created within the same millisecond.
func (a *AppFixtureLifecycle) makeSuffix() string {
	return fmt.Sprintf("%d_%d_%d", time.Now().UnixMilli(), os.Getpid(), globalSuffixCounter.Add(1))
}

// AppFixtureOptions defines options for configuring app fixture behavior, including fresh retrieval and filtering profiles.
type AppFixtureOptions struct {
	fresh  bool
	filter func(p any) bool
}

// AppFixtureOption represents a functional option for configuring AppFixtureOptions in a flexible and extensible manner.
type AppFixtureOption func(a *AppFixtureOptions)

// WithFresh returns an AppFixtureOption that requests a freshly created fixture instance (new database/keyspace) rather than the shared one.
func WithFresh() AppFixtureOption {
	return func(a *AppFixtureOptions) {
		a.fresh = true
	}
}

// WithFilter applies a filter function that determines if a value of type P should be included in the AppFixtureOptions configuration.
func WithFilter[P comparable](predicate func(p P) bool) AppFixtureOption {
	return func(a *AppFixtureOptions) {
		a.filter = func(p any) bool {
			return predicate(p.(P))
		}
	}
}

// AppFixtureRegistry is a thread-safe registry for managing application test fixtures across multiple profiles.
type AppFixtureRegistry[P comparable] struct {
	lock     sync.Mutex
	profiles map[P]bool
	cache    map[P]*AppFixtureLifecycle
	closers  []Closer
	factory  func(profile P) func(suffix string) AppFixture
}

// NewAppFixtureRegistry initializes and returns a registry for managing test app fixtures based on provided profiles and factory.
func NewAppFixtureRegistry[P comparable](profiles []P, factory func(profile P) func(suffix string) AppFixture) *AppFixtureRegistry[P] {

	profileMap := map[P]bool{}

	for _, p := range profiles {
		profileMap[p] = true
	}

	return &AppFixtureRegistry[P]{
		cache:    make(map[P]*AppFixtureLifecycle),
		profiles: profileMap,
		factory:  factory,
	}
}

// GetFixtures retrieves a list of application fixtures based on provided options, applying filters and lifecycle strategies.
func (r *AppFixtureRegistry[P]) GetFixtures(opts ...AppFixtureOption) []AppFixture {

	// We could do a RWMutex here, but this seems simpler and reasonable
	r.lock.Lock()
	defer r.lock.Unlock()

	var res []AppFixture

	o := &AppFixtureOptions{
		// Our default filter is all
		filter: func(_ any) bool { return true },
	}

	for _, opt := range opts {
		opt(o)
	}

	for p := range r.profiles {
		if o.filter(p) {
			l, ok := r.cache[p]
			if !ok {
				l = NewAppFixtureLifecycle(r.factory(p))
				r.cache[p] = l
			}
			var f AppFixture
			var cl Closer
			if o.fresh {
				f, cl = l.GetFresh()
				res = append(res, f)
			} else {
				f, cl = l.GetShared()
				res = append(res, f)
			}
			if cl != nil {
				r.closers = append(r.closers, cl)
			}
		}
	}

	return res
}

// Cleanup releases all resources registered in the registry by invoking their closers. Panics if any closer returns an error.
func (r *AppFixtureRegistry[P]) Cleanup() {
	for _, c := range r.closers {
		if err := c(); err != nil {
			panic(err)
		}
	}
}
