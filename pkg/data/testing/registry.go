package testing

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// AppFixtureLifecycle manages the lifecycle of application test fixtures, supporting eager and lazy initialization strategies.
type AppFixtureLifecycle struct {
	once  *sync.Once
	eager AppFixture
	lazy  func(suffix string) AppFixture
}

// NewAppFixtureLifecycle creates a new AppFixtureLifecycle instance using the provided factory for lazy initialization.
func NewAppFixtureLifecycle(factory func(suffix string) AppFixture) *AppFixtureLifecycle {
	return &AppFixtureLifecycle{
		lazy: factory,
		once: &sync.Once{},
	}
}

// Closer defines a function type responsible for releasing resources or performing cleanup operations, returning an error if any.
type Closer func() error

// GetEager retrieves or initializes the eager AppFixture instance and its corresponding Closer for resource cleanup.
func (a *AppFixtureLifecycle) GetEager() (AppFixture, Closer) {

	var cl Closer
	a.once.Do(func() {
		// Only return a closer if we got the instance from a lazy call
		a.eager, cl = a.GetLazy()
	})

	return a.eager, cl
}

// GetLazy creates a new AppFixture instance with a unique suffix and returns the fixture and its cleanup function.
func (a *AppFixtureLifecycle) GetLazy() (AppFixture, Closer) {
	f := a.lazy(a.makeSuffix())
	return f, f.Cleanup
}

// makeSuffix generates a unique suffix string based on the current timestamp in milliseconds and the process ID.
func (a *AppFixtureLifecycle) makeSuffix() string {
	return fmt.Sprintf("%d_%d", time.Now().UnixMilli(), os.Getpid())
}

// AppFixtureOptions defines options for configuring app fixture behavior, including lazy loading and filtering profiles.
type AppFixtureOptions struct {
	lazy   bool
	filter func(p any) bool
}

// AppFixtureOption represents a functional option for configuring AppFixtureOptions in a flexible and extensible manner.
type AppFixtureOption func(a *AppFixtureOptions)

// WithLazy returns an AppFixtureOption that enables lazy initialization for AppFixtureOptions.
func WithLazy() AppFixtureOption {
	return func(a *AppFixtureOptions) {
		a.lazy = true
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
			if o.lazy {
				f, cl = l.GetLazy()
				res = append(res, f)
			} else {
				f, cl = l.GetEager()
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
