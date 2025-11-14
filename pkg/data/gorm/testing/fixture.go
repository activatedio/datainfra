package testing

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/activatedio/datainfra/pkg/data"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/activatedio/datainfra/pkg/migrate"
	gormmigrate "github.com/activatedio/datainfra/pkg/migrate/gorm"
	"github.com/activatedio/datainfra/pkg/setup"
	gormsetup "github.com/activatedio/datainfra/pkg/setup/gorm"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

type contextProvider struct {
	contextBuilder data.ContextBuilder
}

func (c *contextProvider) GetContext() context.Context {
	return c.contextBuilder.Build(context.Background())
}

func NewContextProvider(contextBuilder data.ContextBuilder) datatesting.ContextProvider {
	return &contextProvider{
		contextBuilder: contextBuilder,
	}
}

type appFixture struct {
	mu       sync.Mutex
	closer   func() error
	migrated bool
	name     string
	opt      fx.Option
}

func (a *appFixture) Cleanup() error {
	if a.closer != nil {
		return a.closer()
	}
	return nil
}

type InvokeParams struct {
	fx.In
	Setup    setup.Setup
	Migrator migrate.Migrator
}

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

func NewAppFixture(name string, opt fx.Option) datatesting.AppFixture {
	return &appFixture{
		name: fmt.Sprintf("gorm: %s", name),
		opt:  opt,
	}
}
