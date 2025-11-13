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
	BeforeAll()
	GetApp(t *testing.T, toInvoke any, toProvide ...any) AppFixtureResult
	AfterAll()
}

type ContextProvider interface {
	GetContext() context.Context
}
