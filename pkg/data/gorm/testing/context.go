package testing

import (
	"context"
	"sync"

	"github.com/activatedio/datainfra/pkg/data"
	"github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/data/testing"
)

type contextProvider struct {
	contextBuilder data.ContextBuilder
	ctx            context.Context
	once           *sync.Once
}

// GetContext returns a context.Context instance.
func (c *contextProvider) GetContext() context.Context {
	c.once.Do(func() {
		c.ctx = c.contextBuilder.Build(context.Background())
	})
	return c.ctx
}

// AfterTest closes the database connection.
func (c *contextProvider) AfterTest() error {
	if c.ctx != nil {
		db, err := gorm.GetDB(c.ctx)
		if err != nil {
			return err
		}
		_db, err := db.DB()
		if err != nil {
			return err
		}
		return _db.Close()
	}
	return nil
}

// NewContextProvider returns a new ContextProvider.
func NewContextProvider(contextBuilder data.ContextBuilder) testing.ContextProvider {
	return &contextProvider{
		contextBuilder: contextBuilder,
		once:           &sync.Once{},
	}
}
