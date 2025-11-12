package data

import (
	"context"
	// TODO - can we use another interface type to mock what is needed here?
	"k8s.io/apimachinery/pkg/labels"
)

// None represents a type used as a placeholder or marker when no meaningful value or identifier is required.
type None struct {
}

// List represents a paginated collection of items of type E.
type List[E any] struct {
	NextPageToken string
	List          []E
}

// PageParams defines pagination parameters for listing or searching operations.
// It includes a page token for navigating to a specific page, and a count for the number of items to include per page.
type PageParams struct {
	PageToken string
	Count     int
}

// Scope is a generic type that can represent any context-specific or application-wide scope definition.
type Scope any

// ContextBuilder defines a contract for building and returning a new context derived from an existing context.
type ContextBuilder interface {
	Build(ctx context.Context) context.Context
}

// ScopeTemplate is a generic interface for defining and retrieving scope-specific context configurations.
type ScopeTemplate[S Scope] interface {
	CurrentScope(ctx context.Context) S
}

// FindByKeyTemplate provides functionality to locate entities by their key and check their existence in a data store.
type FindByKeyTemplate[E any, K comparable] interface {
	FindByKey(ctx context.Context, key K) (E, error)
	ExistsByKey(ctx context.Context, key K) (bool, error)
}

// ListParams specifies parameters for filtering and paginating results in a list operation.
type ListParams struct {
	PageParams *PageParams
	Selector   labels.Selector
}

// ListAllTemplate defines an interface for listing all entities of type E, with support for context and parameters.
type ListAllTemplate[E any] interface {
	ListAll(ctx context.Context, params ListParams) (*List[E], error)
}

// CrudTemplate defines a generic CRUD interface for managing entities of type E with key of type K in a data store.
type CrudTemplate[E any, K comparable] interface {
	FindByKeyTemplate[E, K]
	ListAllTemplate[E]
	Create(ctx context.Context, entity E) error
	Update(ctx context.Context, entity E) error
	Delete(ctx context.Context, key K) error
	DeleteEntity(ctx context.Context, entity E) error
}

// FilterKeysTemplate is a generic interface for filtering keys of type K within a given context.
// It verifies key existence based on the current scope to ensure data integrity and access control.
type FilterKeysTemplate[K comparable] interface {
	// FilterKeys - given a list of keys, returns which exist given the current scope. Useful for data integrity
	// and access control
	FilterKeys(ctx context.Context, keys []K) ([]K, error)
}

// AssociateParentRepository defines a generic interface for checking the existence of a parent entity by its key.
type AssociateParentRepository[K comparable] interface {
	ExistsByKey(ctx context.Context, key K) (bool, error)
}

// AssociateChildRepository defines an interface for filtering child keys based on provided criteria in a repository context.
type AssociateChildRepository[K comparable] interface {
	FilterKeys(ctx context.Context, keys []K) ([]K, error)
}
