package testing

import (
	"context"
	"testing"

	"go.uber.org/fx/fxtest"
)

// AppFixtureResult represents the result of setting up a test fixture, including the app instance and its name.
type AppFixtureResult struct {
	Name string
	App  *fxtest.App
}

// AppFixture defines an interface for managing test fixtures providing application setups and cleanup functionalities.
type AppFixture interface {
	// GetApp configures and retrieves an application fixture for testing based on provided dependencies and invocation.
	GetApp(t *testing.T, toInvoke any, toProvide ...any) AppFixtureResult
	// Cleanup ensures proper teardown of resources associated with the fixture.
	Cleanup() error
}

// ContextProvider is an interface that provides a method to retrieve a context.Context instance.
type ContextProvider interface {
	GetContext() context.Context
}
