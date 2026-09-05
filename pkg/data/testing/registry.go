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
// Reuse and ReuseWithMigrate borrow, and any number of fresh ones.
type AppFixtureLifecycle struct {
	once    *sync.Once
	shared  LifecycleFixture
	factory func(suffix string) LifecycleFixture
}

func NewAppFixtureLifecycle(factory func(suffix string) LifecycleFixture) *AppFixtureLifecycle {
	return &AppFixtureLifecycle{
		factory: factory,
		once:    &sync.Once{},
	}
}

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

// GetFresh returns a new fixture with its own suffix. Its database is dropped
// by the fixture itself when the test that used it ends, so there is no
// Closer to hold.
func (a *AppFixtureLifecycle) GetFresh() LifecycleFixture {
	return a.factory(a.makeSuffix())
}

func (a *AppFixtureLifecycle) makeSuffix() string {
	return fmt.Sprintf("%d_%d_%d", time.Now().UnixMilli(), os.Getpid(), globalSuffixCounter.Add(1))
}

type AppFixtureOptions struct {
	mode   Mode
	filter func(p any) bool
}

type AppFixtureOption func(a *AppFixtureOptions)

// WithMode selects the lifecycle for the returned fixtures. The default is
// ModeReuse.
func WithMode(mode Mode) AppFixtureOption {
	return func(a *AppFixtureOptions) {
		a.mode = mode
	}
}

func WithFilter[P comparable](predicate func(p P) bool) AppFixtureOption {
	return func(a *AppFixtureOptions) {
		a.filter = func(p any) bool {
			return predicate(p.(P))
		}
	}
}

type AppFixtureRegistry[P comparable] struct {
	lock     sync.Mutex
	profiles map[P]bool
	cache    map[P]*AppFixtureLifecycle
	closers  []Closer
	factory  func(profile P) func(suffix string) LifecycleFixture
}

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

// GetFixtures returns one Mode-bound fixture per matching profile.
func (r *AppFixtureRegistry[P]) GetFixtures(opts ...AppFixtureOption) []AppFixture {

	r.lock.Lock()
	defer r.lock.Unlock()

	var res []AppFixture

	o := &AppFixtureOptions{
		mode:   ModeReuse,
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
		if o.mode == ModeFresh {
			res = append(res, Bind(l.GetFresh(), o.mode))
			continue
		}
		f, cl := l.GetShared()
		if cl != nil {
			r.closers = append(r.closers, cl)
		}
		res = append(res, Bind(f, o.mode))
	}

	return res
}

// Cleanup drops every shared database. Fresh databases and per-test deltas
// were already undone by their own t.Cleanup. Call it after m.Run.
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
