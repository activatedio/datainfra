package gorm

import (
	"context"

	"github.com/activatedio/datainfra/pkg/data"
	"gorm.io/gorm"
)

// ContextScope defines a mechanism to apply scoped queries and inject contextual values into entities.
type ContextScope struct {
	// QueryModifier customizes database queries using context-specific rules based on the provided gorm.DB instance.
	QueryModifier func(*gorm.DB) *gorm.DB
	// ValueInjector specifies how to modify or enhance an entity with context-derived information.
	ValueInjector func(e any)
}

// ContextScopeFactory defines a function type for creating a ContextScope with contextual query modifiers and value injectors.
type ContextScopeFactory func(ctx context.Context, table string, fetchType data.FetchType) *ContextScope
