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

// appFixture is a struct that manages test application setup, state, and clean-up procedures for testing purposes.
type appFixture struct {
	once     sync.Once
	setupErr error
	closer   func() error
	name     string
	opt      fx.Option
}

// Cleanup releases resources associated with the appFixture by invoking the closer function, if it is not nil.
func (a *appFixture) Cleanup() error {
	if a.closer != nil {
		return a.closer()
	}
	return nil
}

// InvokeParams provides dependencies for invocation, including setup and migration components via fx.In.
type InvokeParams struct {
	fx.In
	Setup       setup.Setup
	Migrator    migrate.Migrator
	SetupParams *setup.Params `optional:"true"`
}

// GetApp initializes a test application instance with provided dependencies and invokes setup, returning a result object.
func (a *appFixture) GetApp(_ *testing.T, provide ...any) datatesting.AppFixtureResult {

	invoke := make([]any, 0, 1)

	invoke = append(invoke, func(ip InvokeParams) error {

		sp := setup.Params{FailOnExisting: true}

		if ip.SetupParams != nil {
			sp = *ip.SetupParams
		}

		a.once.Do(func() {

			ctx := context.Background()

			if ip.Setup != nil {
				log.Info().Msg("running setup")
				setupStart := time.Now()
				if err := ip.Setup.Setup(ctx, sp); err != nil {
					a.setupErr = err
					return
				}
				log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(setupStart).String()).Msg("setup complete")
				a.closer = func() error {
					teardownStart := time.Now()
					err := ip.Setup.Teardown(ctx)
					log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(teardownStart).String()).Msg("teardown complete")
					return err
				}
			}

			if ip.Migrator != nil {
				migrateStart := time.Now()
				if err := ip.Migrator.Migrate(ctx); err != nil {
					a.setupErr = err
					return
				}
				log.Info().Str("component", "gorm").Str("fixture", a.name).Str("duration", time.Since(migrateStart).String()).Msg("migration complete")
			}
		})

		return a.setupErr
	})

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
		// This is the test itself
		fx.Invoke(invoke...),
	)

	return datatesting.AppFixtureResult{
		App:  app,
		Name: a.name,
	}
}

// NewAppFixture creates a new AppFixture for testing, initializing it with a name and an fx.Option configuration.
func NewAppFixture(name string, opt fx.Option) datatesting.AppFixture {
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
