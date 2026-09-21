package testing

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var globalSuffixCounter atomic.Int64

// AppFixtureLifecycle owns one profile's fixtures: the single shared one that
// most tests borrow, and any number of dedicated ones.
type AppFixtureLifecycle struct {
	once    *sync.Once
	shared  LifecycleFixture
	factory func(suffix string) LifecycleFixture
}

// NewAppFixtureLifecycle returns a lifecycle that mints fixtures through
// factory, each with a suffix unique to this process and call.
func NewAppFixtureLifecycle(factory func(suffix string) LifecycleFixture) *AppFixtureLifecycle {
	return &AppFixtureLifecycle{
		factory: factory,
		once:    &sync.Once{},
	}
}

// Closer drops a shared database. It is handed out exactly once per shared
// fixture, to whoever created it, so a suite tears down what it built and
// nothing tears down what it merely borrowed.
type Closer func() error

// GetShared returns the profile's shared fixture, creating it on first use.
// The Closer is non-nil only on that first call: the shared database is
// dropped once, at suite end, by whoever holds it.
func (a *AppFixtureLifecycle) GetShared() (LifecycleFixture, Closer) {
	var cl Closer
	a.once.Do(func() {
		a.shared = a.factory(a.makeSuffix())
		cl = a.shared.Cleanup
	})
	return a.shared, cl
}

// GetDedicated returns a new fixture with its own suffix. Its database is
// dropped by the fixture itself when the test that used it ends, so there is
// no Closer to hold.
func (a *AppFixtureLifecycle) GetDedicated() LifecycleFixture {
	return a.factory(a.makeSuffix())
}

func (a *AppFixtureLifecycle) makeSuffix() string {
	return fmt.Sprintf("%d_%d_%d", time.Now().UnixMilli(), os.Getpid(), globalSuffixCounter.Add(1))
}

// AppFixtureOptions is what the AppFixtureOption functions accumulate into:
// what the caller requires of a fixture, and which profiles it wants.
type AppFixtureOptions struct {
	req    Requirement
	filter func(p any) bool
}

// AppFixtureOption narrows what GetFixtures returns. See Require and
// WithFilter.
type AppFixtureOption func(a *AppFixtureOptions)

// Require states what the returned fixtures must provide. The default is the
// whole stack, Tolerant, shared.
func Require(req Requirement) AppFixtureOption {
	return func(a *AppFixtureOptions) {
		a.req = req
	}
}

// WithFilter restricts the returned fixtures to the profiles the predicate
// accepts. P must be the registry's own profile type; a mismatch panics on
// the type assertion, which is a wiring error rather than a test failure.
func WithFilter[P comparable](predicate func(p P) bool) AppFixtureOption {
	return func(a *AppFixtureOptions) {
		a.filter = func(p any) bool {
			return predicate(p.(P))
		}
	}
}

// AppFixtureRegistry hands out fixtures per profile, creating each profile's
// lifecycle on first use and remembering the closers for the shared
// databases so Cleanup can drop them at suite end.
type AppFixtureRegistry[P comparable] struct {
	lock     sync.Mutex
	profiles map[P]bool
	cache    map[P]*AppFixtureLifecycle
	closers  []Closer
	factory  func(profile P) func(suffix string) LifecycleFixture
}

// NewAppFixtureRegistry returns a registry over profiles, building each
// profile's fixture factory on demand rather than up front, so a suite that
// filters to one profile never stands up the others.
func NewAppFixtureRegistry[P comparable](profiles []P, factory func(profile P) func(suffix string) LifecycleFixture) *AppFixtureRegistry[P] {

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

// GetFixtures returns one Requirement-bound fixture per matching profile.
func (r *AppFixtureRegistry[P]) GetFixtures(opts ...AppFixtureOption) []AppFixture {

	r.lock.Lock()
	defer r.lock.Unlock()

	res := make([]AppFixture, 0, len(r.profiles))

	o := &AppFixtureOptions{
		filter: func(_ any) bool { return true },
	}

	for _, opt := range opts {
		opt(o)
	}

	for p := range r.profiles {
		if !o.filter(p) {
			continue
		}
		l, ok := r.cache[p]
		if !ok {
			l = NewAppFixtureLifecycle(r.factory(p))
			r.cache[p] = l
		}
		if o.req.Dedicated {
			res = append(res, Bind(l.GetDedicated(), o.req))
			continue
		}
		f, cl := l.GetShared()
		if cl != nil {
			r.closers = append(r.closers, cl)
		}
		res = append(res, Bind(f, o.req))
	}

	return res
}

// Cleanup drops every shared database. Dedicated databases were already
// dropped by their own t.Cleanup. Call it after m.Run.
func (r *AppFixtureRegistry[P]) Cleanup() {
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, c := range r.closers {
		if err := c(); err != nil {
			panic(err)
		}
	}
	r.closers = nil
}
