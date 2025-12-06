package testing

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/activatedio/datainfra/pkg/data"
	gorm2 "github.com/activatedio/datainfra/pkg/data/gorm"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/activatedio/datainfra/pkg/migrate"
	gormmigrate "github.com/activatedio/datainfra/pkg/migrate/gorm"
	"github.com/activatedio/datainfra/pkg/setup"
	gormsetup "github.com/activatedio/datainfra/pkg/setup/gorm"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// contextProvider is a struct implementing the ContextProvider interface using a data.ContextBuilder.
type contextProvider struct {
	contextBuilder data.ContextBuilder
}

// GetContext builds and returns a new context derived from a background context using the context builder.
func (c *contextProvider) GetContext() context.Context {
	return c.contextBuilder.Build(context.Background())
}

// NewContextProvider creates a datatesting.ContextProvider using the provided data.ContextBuilder to manage contexts.
func NewContextProvider(contextBuilder data.ContextBuilder) datatesting.ContextProvider {
	return &contextProvider{
		contextBuilder: contextBuilder,
	}
}

// appFixture is a struct that manages test application setup, state, and clean-up procedures for testing purposes.
type appFixture struct {
	mu       sync.Mutex
	closer   func() error
	migrated bool
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
	Setup    setup.Setup
	Migrator migrate.Migrator
}

// GetApp initializes a test application instance with provided dependencies and invokes setup, returning a result object.
func (a *appFixture) GetApp(t *testing.T, toInvoke any, provide ...any) datatesting.AppFixtureResult {

	a.mu.Lock()

	var invoke []any

	if a.migrated {
		a.mu.Unlock()
	} else {

		invoke = append(invoke, func(ip InvokeParams) error {

			defer a.mu.Unlock()

			if ip.Setup != nil {
				log.Info().Msg("running setup")
				if err := ip.Setup.Setup(setup.Params{FailOnExisting: true}); err != nil {
					return err
				}
				a.closer = func() error {
					return ip.Setup.Teardown()
				}
			}

			if ip.Migrator != nil {
				if err := ip.Migrator.Migrate(); err != nil {
					return err
				}
			}

			return nil

		})
	}

	invoke = append(invoke, toInvoke)

	app := fxtest.New(t, a.opt,
		fx.Provide(NewContextProvider, gormsetup.NewSetup, gormmigrate.NewMigrator),
		fx.Provide(provide...),
		fx.Invoke(invoke...))

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
