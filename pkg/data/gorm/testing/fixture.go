package testing

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"go.uber.org/fx"

	"github.com/activatedio/datainfra/pkg/data"
	gorm2 "github.com/activatedio/datainfra/pkg/data/gorm"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/activatedio/datainfra/pkg/migrate"
	gormmigrate "github.com/activatedio/datainfra/pkg/migrate/gorm"
	"github.com/activatedio/datainfra/pkg/setup"
	gormsetup "github.com/activatedio/datainfra/pkg/setup/gorm"
)

// applied is one layer the store currently carries, with the instance that
// applied it — Down must go through that same instance, because a
// parameterized layer's reverse depends on the parameters it was applied
// with.
type applied struct {
	layer migrate.Layer
	key   string
}

// appFixture is one database — a store — and the planner that brings it to
// what each test requires.
//
// Every test runs through the same rings: Setup (once per store), MigrateUp
// of the layers it lacks, the test, MigrateDown of what the next test does
// not want, Teardown (dedicated stores when the test ends, shared ones at
// suite end). What varies is the Requirement, and the planner turns the
// store's recorded state plus the requirement into the cheapest exact plan.
type appFixture struct {
	name string
	opt  fx.Option

	// setupOnce guards Setup. setupStage is kept so a broken store can be
	// dropped and recreated.
	setupOnce  sync.Once
	setupErr   error
	setupStage setup.Setup
	setupParam setup.Params
	teardown   func() error

	tornMu sync.Mutex
	torn   bool

	// runMu is held exclusively while the planner mutates the store and for
	// the duration of a Pristine test; Tolerant tests hold it shared.
	runMu sync.RWMutex

	// stateMu guards the recorded state below.
	stateMu sync.Mutex
	applied []applied
	dirty   bool
	broken  error
}

// Cleanup drops the database if it is still there. Per-test work never waits
// for this; it is the suite-end drop of a shared store.
func (a *appFixture) Cleanup() error {
	return a.runTeardown()
}

func (a *appFixture) runTeardown() error {
	a.tornMu.Lock()
	defer a.tornMu.Unlock()
	if a.torn || a.teardown == nil {
		return nil
	}
	a.torn = true
	return a.teardown()
}

// BaseParams is what Setup needs.
type BaseParams struct {
	fx.In
	Setup       setup.Setup
	SetupParams *setup.Params `optional:"true"`
}

// StackParams is the fixture's declared stack, in order. Layers are
// constructed after Setup has run, so they may depend on the pool.
type StackParams struct {
	fx.In
	Stack []migrate.Layer
}

// GetApp returns the fx option for one test's app against this store. Setup
// and the plan run inside the app's invokes, in registration order, before
// anything the caller registers after this option.
func (a *appFixture) GetApp(t *testing.T, req datatesting.Requirement, provide ...any) datatesting.AppFixtureResult {

	base := func(bp BaseParams) error {
		a.setupOnce.Do(func() {
			a.setupStage = bp.Setup
			a.setupParam = setup.Params{FailOnExisting: true}
			if bp.SetupParams != nil {
				a.setupParam = *bp.SetupParams
			}
			a.setupErr = a.create(context.Background())
			if a.setupErr != nil {
				return
			}
			// A dedicated store belongs to this test: drop it when the test
			// ends, whether it passed, failed or was skipped.
			if req.Dedicated {
				t.Cleanup(func() {
					if err := a.runTeardown(); err != nil {
						t.Errorf("fixture %s: teardown: %v", a.name, err)
					}
				})
			}
		})
		return a.setupErr
	}

	stack := func(sp StackParams) error {
		return a.acquire(t, req, sp.Stack)
	}

	app := fx.Module("test", a.opt,
		fx.Provide(func(contextBuilder data.ContextBuilder, lc fx.Lifecycle) datatesting.ContextProvider {
			cp := NewContextProvider(contextBuilder)
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					return cp.AfterTest()
				},
			})
			return cp
		}, gormsetup.NewSetup),
		fx.Provide(provide...),
		fx.Invoke(base),
		fx.Invoke(stack),
	)

	return datatesting.AppFixtureResult{
		App:  app,
		Name: a.name,
	}
}

// create provisions the database and records the teardown.
func (a *appFixture) create(ctx context.Context) error {
	log.Info().Str("fixture", a.name).Msg("running setup")
	start := time.Now()
	if err := a.setupStage.Setup(ctx, a.setupParam); err != nil {
		return err
	}
	a.teardown = func() error {
		teardownStart := time.Now()
		err := a.setupStage.Teardown(ctx)
		log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(teardownStart).String()).Msg("teardown complete")
		return err
	}
	a.tornMu.Lock()
	a.torn = false
	a.tornMu.Unlock()
	log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(start).String()).Msg("setup complete")
	return nil
}

// acquire brings the store to req and holds it for the test: exclusively
// for a Pristine test, shared for a Tolerant one. The hold is released by
// t.Cleanup, which also records that the test may have written.
func (a *appFixture) acquire(t *testing.T, req datatesting.Requirement, stack []migrate.Layer) error {
	ctx := context.Background()
	target, err := resolve(req.Stack, stack)
	if err != nil {
		return fmt.Errorf("fixture %s: %w", a.name, err)
	}

	for {
		a.runMu.Lock()

		if err := a.recoverIfBroken(ctx); err != nil {
			a.runMu.Unlock()
			return fmt.Errorf("fixture %s: %w", a.name, err)
		}

		if err := a.plan(ctx, target, req.Tolerance); err != nil {
			a.stateMu.Lock()
			a.broken = err
			a.stateMu.Unlock()
			a.runMu.Unlock()
			return fmt.Errorf("fixture %s: %w", a.name, err)
		}

		if req.Tolerance == datatesting.Pristine {
			t.Cleanup(func() {
				a.markDirty()
				a.runMu.Unlock()
			})
			return nil
		}

		// Tolerant: downgrade to a shared hold. Another test may take the
		// exclusive lock in the gap and change the stack, so re-check what
		// we hold before trusting it.
		a.runMu.Unlock()
		a.runMu.RLock()
		if a.carries(target) {
			t.Cleanup(func() {
				a.markDirty()
				a.runMu.RUnlock()
			})
			return nil
		}
		a.runMu.RUnlock()
	}
}

// recoverIfBroken drops and recreates a store whose last plan failed
// part-way. The failing test already reported the error; this keeps one bad
// Down from failing every test after it. Called with runMu held.
func (a *appFixture) recoverIfBroken(ctx context.Context) error {
	a.stateMu.Lock()
	broken := a.broken
	a.stateMu.Unlock()
	if broken == nil {
		return nil
	}
	log.Error().Str("fixture", a.name).Err(broken).Msg("store is broken from a failed migration step; dropping and recreating it")
	if err := a.runTeardown(); err != nil {
		return fmt.Errorf("recovering broken store: teardown: %w (broken by: %v)", err, broken)
	}
	if err := a.create(ctx); err != nil {
		return fmt.Errorf("recovering broken store: setup: %w (broken by: %v)", err, broken)
	}
	a.stateMu.Lock()
	a.applied = nil
	a.dirty = false
	a.broken = nil
	a.stateMu.Unlock()
	return nil
}

// plan brings the store from its recorded state to target. Called with runMu
// held exclusively.
//
//  1. Find how many leading layers already match, by name and key.
//  2. If the test needs pristine and the store is dirty: Reset the bottom
//     layer if it can (discarding everything above), else Down everything.
//  3. Down the applied layers above the match, top to bottom.
//  4. Up the missing target layers, bottom to top.
func (a *appFixture) plan(ctx context.Context, target []migrate.Layer, tolerance datatesting.Tolerance) error {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	i := 0
	for i < len(a.applied) && i < len(target) &&
		a.applied[i].layer.Name() == target[i].Name() && a.applied[i].key == migrate.KeyOf(target[i]) {
		i++
	}

	if tolerance == datatesting.Pristine && a.dirty {
		if r, ok := a.applied[0].layer.(migrate.Resettable); i >= 1 && ok {
			if err := a.step("reset", a.applied[0].layer, r.Reset); err != nil {
				return err
			}
			a.applied = a.applied[:1]
			i = 1
		} else {
			for j := len(a.applied) - 1; j >= 0; j-- {
				if err := a.step("down", a.applied[j].layer, a.applied[j].layer.Down); err != nil {
					return err
				}
				a.applied = a.applied[:j]
			}
			i = 0
		}
		a.dirty = false
	}

	for j := len(a.applied) - 1; j >= i; j-- {
		if err := a.step("down", a.applied[j].layer, a.applied[j].layer.Down); err != nil {
			return err
		}
		a.applied = a.applied[:j]
	}

	for j := i; j < len(target); j++ {
		if err := a.step("up", target[j], target[j].Up); err != nil {
			return err
		}
		a.applied = append(a.applied, applied{layer: target[j], key: migrate.KeyOf(target[j])})
	}

	return nil
}

func (a *appFixture) step(direction string, l migrate.Layer, fn func(context.Context) error) error {
	start := time.Now()
	if err := fn(context.Background()); err != nil {
		return fmt.Errorf("layer %q %s: %w", l.Name(), direction, err)
	}
	log.Info().Str("component", "gorm").Str("fixture", a.name).Str("layer", l.Name()).Str("direction", direction).
		Str("duration", time.Since(start).String()).Msg("layer step complete")
	return nil
}

// carries reports whether the store's applied stack is exactly target.
func (a *appFixture) carries(target []migrate.Layer) bool {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if a.broken != nil || len(a.applied) != len(target) {
		return false
	}
	for i := range target {
		if a.applied[i].layer.Name() != target[i].Name() || a.applied[i].key != migrate.KeyOf(target[i]) {
			return false
		}
	}
	return true
}

func (a *appFixture) markDirty() {
	a.stateMu.Lock()
	a.dirty = true
	a.stateMu.Unlock()
}

// resolve picks the requested layer names out of the declared stack, in
// stack order. Nil means the whole stack. A name the stack does not declare,
// or names out of stack order, is a test bug and fails loudly.
func resolve(names []string, stack []migrate.Layer) ([]migrate.Layer, error) {
	if names == nil {
		return stack, nil
	}
	index := map[string]int{}
	for i, l := range stack {
		if _, dup := index[l.Name()]; dup {
			return nil, fmt.Errorf("stack declares layer %q twice", l.Name())
		}
		index[l.Name()] = i
	}
	out := make([]migrate.Layer, 0, len(names))
	last := -1
	for _, n := range names {
		i, ok := index[n]
		if !ok {
			return nil, fmt.Errorf("requirement names layer %q, which the stack does not declare", n)
		}
		if i <= last {
			return nil, fmt.Errorf("requirement lists layer %q out of stack order", n)
		}
		last = i
		out = append(out, stack[i])
	}
	return out, nil
}

// NewAppFixture creates a LifecycleFixture over one database, described by
// the fx option: it must provide the *datagorm.Config, the
// *gormsetup.OwnerGormConfig, and the ordered []migrate.Layer stack.
func NewAppFixture(name string, opt fx.Option) datatesting.LifecycleFixture {
	return &appFixture{
		name: fmt.Sprintf("gorm: %s", name),
		opt:  opt,
	}
}

// GormTestingConfigResult is a struct used to hold configuration results for testing GORM setups.
type GormTestingConfigResult struct {
	fx.Out
	GormConfig         *gorm2.Config
	SetupGormConfig    *gormsetup.OwnerGormConfig
	MigratorGormConfig *gormmigrate.MigratorGormConfig
}

// NewStaticGormTestingConfig creates a static GORM testing configuration function using the provided configs.
func NewStaticGormTestingConfig(ownerConfig, appConfig *gorm2.Config) func() GormTestingConfigResult {
	return func() GormTestingConfigResult {
		return GormTestingConfigResult{
			GormConfig: appConfig,
			SetupGormConfig: &gormsetup.OwnerGormConfig{
				Config: *ownerConfig,
			},
			MigratorGormConfig: &gormmigrate.MigratorGormConfig{
				GormConfig: gorm2.Config{
					Dialect:                  ownerConfig.Dialect,
					EnableDefaultTransaction: ownerConfig.EnableDefaultTransaction,
					EnableSQLLogging:         ownerConfig.EnableSQLLogging,
					Host:                     ownerConfig.Host,
					Port:                     ownerConfig.Port,
					Username:                 ownerConfig.Username,
					Password:                 ownerConfig.Password,
					Name:                     appConfig.Name,
				},
			},
		}
	}
}
