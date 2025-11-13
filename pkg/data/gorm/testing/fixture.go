package testing

import (
	"context"
	"fmt"
	"testing"

	"github.com/activatedio/datainfra/pkg/data"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
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
	name string
	opt  fx.Option
}

func (a *appFixture) BeforeAll() {
	log.Info().Msg("setup db")
}

func (a *appFixture) AfterAll() {
	log.Info().Msg("teardown db")
}

func (a *appFixture) GetApp(t *testing.T, toInvoke any, provide ...any) datatesting.AppFixtureResult {
	var invoke []any
	invoke = append(invoke, func() {
		log.Info().Msg("before test")
	},
	)

	invoke = append(invoke, toInvoke)

	invoke = append(invoke, func() {
		log.Info().Msg("after test")
	},
	)
	app := fxtest.New(t, a.opt,
		fx.Provide(NewContextProvider),
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
