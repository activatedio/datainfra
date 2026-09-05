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

// appFixture is one database. It runs the lifecycle rings for every test that
// borrows it, according to the Mode the test asked for:
//
//	ring 1  Setup + base MigrateUp     once per database (baseOnce)
//	ring 2  delta MigrateUp            once (Reuse, Fresh) or per test (ReuseWithMigrate)
//	        [test]
//	ring 3  delta MigrateDown          per test, ReuseWithMigrate only, via t.Cleanup
//	ring 4  Teardown                   via t.Cleanup (Fresh) or Cleanup at suite end (shared)
type appFixture struct {
	name string
	opt  fx.Option

	baseOnce sync.Once
	baseErr  error
	teardown func() error

	// deltaOnce guards the one-shot delta of Reuse and Fresh. deltaMu
	// serializes ReuseWithMigrate deltas on this database: it is held from
	// Up until the test's Down has run.
	deltaOnce sync.Once
	deltaErr  error
	deltaMu   sync.Mutex

	// torn records that Teardown has run, so a Fresh fixture's suite-end
	// Cleanup (if a caller registers one anyway) is a no-op.
	tornMu sync.Mutex
	torn   bool
}

// Cleanup drops the database if it is still there. Per-test rings never wait
// for this; it is the suite-end drop of a shared database.
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

// BaseParams is ring 1: provisioning and the base migrations.
type BaseParams struct {
	fx.In
	Setup       setup.Setup
	Migrator    migrate.Migrator `optional:"true"`
	SetupParams *setup.Params    `optional:"true"`
}

// DeltaParams is ring 2/3: the per-test reversible migrations, if the graph
// has any.
type DeltaParams struct {
	fx.In
	Delta migrate.Reversible `name:"delta" optional:"true"`
}

// GetApp returns the fx option for one test's app against this database. The
// rings run inside the app's invokes, in registration order: base before
// delta, and delta before anything the caller registers after this option.
// Deltas may depend on the pool (they usually do: a bootstrap loader goes
// through repositories); the base may not, since the database does not exist
// until it has run. Keeping them in two invokes is what enforces that order.
func (a *appFixture) GetApp(t *testing.T, mode datatesting.Mode, provide ...any) datatesting.AppFixtureResult {

	base := func(bp BaseParams) error {
		a.baseOnce.Do(func() {
			ctx := context.Background()
			sp := setup.Params{FailOnExisting: true}
			if bp.SetupParams != nil {
				sp = *bp.SetupParams
			}

			log.Info().Str("fixture", a.name).Str("mode", mode.String()).Msg("running setup")
			setupStart := time.Now()
			if err := bp.Setup.Setup(ctx, sp); err != nil {
				a.baseErr = err
				return
			}
			a.teardown = func() error {
				teardownStart := time.Now()
				err := bp.Setup.Teardown(ctx)
				log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(teardownStart).String()).Msg("teardown complete")
				return err
			}
			log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(setupStart).String()).Msg("setup complete")

			// A fresh database belongs to this test: drop it when the
			// test ends, whether it passed, failed or was skipped.
			if mode == datatesting.ModeFresh {
				t.Cleanup(func() {
					if err := a.runTeardown(); err != nil {
						t.Errorf("fixture %s: teardown: %v", a.name, err)
					}
				})
			}

			if bp.Migrator != nil {
				migrateStart := time.Now()
				if err := bp.Migrator.Migrate(ctx); err != nil {
					a.baseErr = err
					return
				}
				log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(migrateStart).String()).Msg("migration complete")
			}
		})
		return a.baseErr
	}

	delta := func(dp DeltaParams) error {
		if dp.Delta == nil {
			return nil
		}
		ctx := context.Background()
		switch mode {
		case datatesting.ModeReuse, datatesting.ModeFresh:
			a.deltaOnce.Do(func() {
				a.deltaErr = a.deltaUp(ctx, dp.Delta)
			})
			return a.deltaErr
		case datatesting.ModeReuseWithMigrate:
			a.deltaMu.Lock()
			if err := a.deltaUp(ctx, dp.Delta); err != nil {
				// Up may have half-applied; give Down its chance before
				// handing the database to the next test.
				if derr := dp.Delta.Down(ctx); derr != nil {
					log.Warn().Str("fixture", a.name).Err(derr).Msg("delta down after failed up")
				}
				a.deltaMu.Unlock()
				return err
			}
			t.Cleanup(func() {
				defer a.deltaMu.Unlock()
				downStart := time.Now()
				if err := dp.Delta.Down(ctx); err != nil {
					t.Errorf("fixture %s: delta down: %v", a.name, err)
					return
				}
				log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(downStart).String()).Msg("delta down complete")
			})
			return nil
		default:
			return fmt.Errorf("fixture %s: unknown mode %d", a.name, mode)
		}
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
		}, gormsetup.NewSetup, gormmigrate.NewMigrator),
		fx.Provide(provide...),
		fx.Invoke(base),
		fx.Invoke(delta),
	)

	return datatesting.AppFixtureResult{
		App:  app,
		Name: a.name,
	}
}

func (a *appFixture) deltaUp(ctx context.Context, d migrate.Reversible) error {
	start := time.Now()
	if err := d.Up(ctx); err != nil {
		return err
	}
	log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(start).String()).Msg("delta up complete")
	return nil
}

// NewAppFixture creates a LifecycleFixture over one database, described by
// the fx option: it must provide the *datagorm.Config, the
// *gormsetup.OwnerGormConfig, the *gormmigrate.MigratorGormConfig and the
// []gormmigrate.MigratorData of the base tier, and may provide a
// migrate.Reversible tagged name:"delta".
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
	MigratorData       []gormmigrate.MigratorData
}

// NewStaticGormTestingConfig creates a static GORM testing configuration function using the provided configs and migrator data.
func NewStaticGormTestingConfig(ownerConfig, appConfig *gorm2.Config, migratorData []gormmigrate.MigratorData) func() GormTestingConfigResult {
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
			MigratorData: migratorData,
		}
	}
}
