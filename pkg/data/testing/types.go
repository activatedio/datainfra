package testing

import (
	"context"
	"testing"

	"go.uber.org/fx"
)

// AppFixtureResult represents the result of setting up a test fixture, including the app instance and its name.
type AppFixtureResult struct {

	// Name is the name of the fixture
	Name string

	// App is the application option, usually an fx.Module
	App fx.Option
}

// AppFixture defines an interface for managing test fixtures providing application setups and cleanup functionalities.
type AppFixture interface {

	// GetApp configures and retrieves an application fixture for testing based on provided dependencies and invocation.
	GetApp(t *testing.T, toProvide ...any) AppFixtureResult

	// Cleanup ensures proper teardown of resources associated with the fixture.
	Cleanup() error
}

// ContextProvider is an interface that provides a method to retrieve a context.Context instance.
type ContextProvider interface {

	// GetContext returns a context.Context instance.
	GetContext() context.Context

	// AfterTest closes the database connection.
	AfterTest() error
}
