package data

import (
	"context"

	// TODO - can we use another interface type to mock what is needed here?
	"k8s.io/apimachinery/pkg/labels"
)

// SearchOperator is an operator for searching
type SearchOperator int

const (
	// SearchOperatorStringEquals represents the equals operator for string comparisons.
	SearchOperatorStringEquals SearchOperator = iota
	// SearchOperatorStringNotEquals represents the not equals operator for string comparisons.
	SearchOperatorStringNotEquals
	// SearchOperatorStringMatch represents the match operator for string comparisons.
	SearchOperatorStringMatch
	// SearchOperatorStringIn represents the in operator for string comparisons.
	SearchOperatorStringIn
	// SearchOperatorStringNotIn represents the not in operator for string comparisons.
	SearchOperatorStringNotIn
	// SearchOperatorNumberEquals represents the equals operator for number comparisons.
	SearchOperatorNumberEquals
	// SearchOperatorNumberNotEquals represents the not equals operator for number comparisons.
	SearchOperatorNumberNotEquals
	// SearchOperatorNumberIn represents the in operator for number comparisons.
	SearchOperatorNumberIn
)

// SearchPredicateValueType represents a type of the predicate
type SearchPredicateValueType int

const (
	// SearchPredicateValueTypeString represents the string type for predicate values.
	SearchPredicateValueTypeString SearchPredicateValueType = iota
	// SearchPredicateValueTypeNumber represents the number type for predicate values.
	SearchPredicateValueTypeNumber
	// SearchPredicateValueTypeStringArray represents the string array type for predicate values.
	SearchPredicateValueTypeStringArray
	// SearchPredicateValueTypeNumberArray represents the number array type for predicate values.
	SearchPredicateValueTypeNumberArray
)

var searchPredicateValueTypes = map[SearchOperator]SearchPredicateValueType{
	SearchOperatorStringEquals:    SearchPredicateValueTypeString,
	SearchOperatorStringNotEquals: SearchPredicateValueTypeString,
	SearchOperatorStringMatch:     SearchPredicateValueTypeString,
	SearchOperatorStringIn:        SearchPredicateValueTypeStringArray,
	SearchOperatorStringNotIn:     SearchPredicateValueTypeStringArray,
	SearchOperatorNumberEquals:    SearchPredicateValueTypeNumber,
	SearchOperatorNumberNotEquals: SearchPredicateValueTypeNumber,
	SearchOperatorNumberIn:        SearchPredicateValueTypeNumberArray,
}

// GetValueType retrieves the SearchPredicateValueType associated with the SearchOperator.
func (s SearchOperator) GetValueType() SearchPredicateValueType {
	return searchPredicateValueTypes[s]
}

// SearchResult represents the result of a search query containing the entity and its relevance score.
type SearchResult[E any] struct {
	// Score represents the relative relevance of the search result.
	Score float32
	// Entity is the entity that matches the search criteria.
	Entity E
}

// SearchPredicateDescriptor represents a descriptor for a search predicate with a name, label, and allowed operators.
type SearchPredicateDescriptor struct {
	Name      string
	Label     string
	Operators []SearchOperator
}

// SearchPredicate represents a search condition with a name, operator, and value(s) for filtering results.
type SearchPredicate struct {
	Name     string
	Operator SearchOperator
	// Search values - one of supported
	StringValue      string
	NumberValue      float64
	StringArrayValue []string
	NumberArrayValue []float64
}

// SearchTemplate defines an interface for executing search operations and retrieving search predicate information.
type SearchTemplate[E any] interface {
	// Search performs a search operation with the given criteria and paging parameters.
	Search(ctx context.Context, criteria []*SearchPredicate, pageParams *PageParams) (*List[*SearchResult[E]], error)
	// GetSearchPredicates returns a list of available search predicates for filtering results.
	GetSearchPredicates(ctx context.Context) ([]*SearchPredicateDescriptor, error)
}

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
