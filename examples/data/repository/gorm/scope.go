package gorm

import (
	"context"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/pkg/data"
	"github.com/activatedio/datainfra/pkg/data/gorm"
	gorm1 "gorm.io/gorm"
)

// WithTenantScope returns a ContextScopeFactory that applies a tenant filter to all queries.
func WithTenantScope() gorm.ContextScopeFactory {
	return func(ctx context.Context, _ string, _ data.FetchType) *gorm.ContextScope {
		return &gorm.ContextScope{
			QueryModifier: func(db *gorm1.DB) *gorm1.DB {
				return db.Where("tenant_id = ?", model.MustGetTenant(ctx))
			},
			ValueInjector: func(e any) {
				e.(model.TenantScoped).SetTenantID(model.MustGetTenant(ctx))
			},
		}
	}
}
