package model

import "context"

type tenantKey struct{}

// WithTenant returns a new context with the tenant set.
func WithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenant)
}

// MustGetTenant returns the tenant ID from the context. Panics if the tenant is not set.
func MustGetTenant(ctx context.Context) string {
	return ctx.Value(tenantKey{}).(string)
}

// TenantScoped is an interface for entities that are scoped to a tenant.
type TenantScoped interface {
	SetTenantID(id string)
}
