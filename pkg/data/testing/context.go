package testing

import (
	"context"

	"github.com/activatedio/datainfra/pkg/data"
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
func NewContextProvider(contextBuilder data.ContextBuilder) ContextProvider {
	return &contextProvider{
		contextBuilder: contextBuilder,
	}
}
