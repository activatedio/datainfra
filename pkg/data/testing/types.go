package testing

import (
	"context"
	"testing"

	"go.uber.org/fx/fxtest"
)

type AppFixtureResult struct {
	Name string
	App  *fxtest.App
}

type AppFixture interface {
	GetApp(t *testing.T, toInvoke any, toProvide ...any) AppFixtureResult
	Cleanup() error
}

type ContextProvider interface {
	GetContext() context.Context
}
