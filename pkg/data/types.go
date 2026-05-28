package data

import (
	"context"
	"time"

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
//
// Virtual marks predicates that do not correspond to a real domain field —
// for example, full-text search predicates that span multiple columns. The
// convention is to name virtual predicates with a leading "@" (e.g.
// "@keywords") so clients without access to the bool can still recognize
// them on the wire; the bool exists for programmatic dispatch.
type SearchPredicateDescriptor struct {
	Name      string
	Label     string
	Virtual   bool
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

// HistogramIntervalUnit names a calendar/duration unit for histogram
// bucketing. HistogramIntervalAuto defers the choice to the backend,
// which picks the smallest unit that yields ~30–100 buckets across the
// requested [min, max] window.
type HistogramIntervalUnit int

const (
	// HistogramIntervalAuto lets the backend pick the unit based on [min, max].
	HistogramIntervalAuto HistogramIntervalUnit = iota
	// HistogramIntervalMinute buckets by minute(s). Step >1 produces fixed-width buckets.
	HistogramIntervalMinute
	// HistogramIntervalHour buckets by hour(s). Step >1 produces fixed-width buckets.
	HistogramIntervalHour
	// HistogramIntervalDay buckets by calendar day.
	HistogramIntervalDay
	// HistogramIntervalWeek buckets by calendar week.
	HistogramIntervalWeek
	// HistogramIntervalMonth buckets by calendar month.
	HistogramIntervalMonth
)

// HistogramInterval is the bucket-width specification for a histogram
// query. Step is the number of Unit per bucket; 0 or 1 both mean one
// Unit per bucket. Step is meaningful for Minute and Hour (fixed-width
// buckets like 5m, 3h); it is ignored for Day, Week, and Month, which
// always use calendar boundaries.
type HistogramInterval struct {
	Unit HistogramIntervalUnit
	Step int
}

// HistogramSpec is the request payload for a date-histogram aggregation.
// Min and Max bound the window inclusively; the backend returns one
// bucket per Interval slice across [Min, Max], including empty buckets.
type HistogramSpec struct {
	Interval HistogramInterval
	Min      time.Time
	Max      time.Time
}

// HistogramBucket is a single time-slice count.
type HistogramBucket struct {
	Key   time.Time
	Count int64
}

// HistogramResult is the response payload for a date-histogram query.
// ResolvedInterval echoes the unit/step the backend actually used —
// when the caller asked for HistogramIntervalAuto this is the unit the
// backend picked, so the client can label buckets without re-running
// the resolver locally.
type HistogramResult struct {
	ResolvedInterval HistogramInterval
	Buckets          []*HistogramBucket
}

// SearchHistogramTemplate is the histogram counterpart to SearchTemplate.
// It is intentionally separate so a backend can implement search without
// implementing histogram (e.g. gorm today only supports filtered search).
type SearchHistogramTemplate[E any] interface {
	// SearchHistogram applies the same predicate filter as Search and
	// returns one bucket count per slice across [spec.Min, spec.Max].
	SearchHistogram(ctx context.Context, criteria []*SearchPredicate, spec *HistogramSpec) (*HistogramResult, error)
}

// ResolveAutoInterval picks the smallest calendar/fixed unit that yields
// at most targetBuckets across [min, max]. The ladder mirrors what
// Kibana does in its date-histogram defaults so dashboards stay
// readable as the time window widens:
//
//	1m → 5m → 10m → 30m → 1h → 3h → 12h → 1d → 1w → 1mo
//
// targetBuckets defaults to 50 when 0 or negative. Min/Max ordering is
// not enforced — callers can pass them in either order and the
// resolver will use the absolute duration. When min == max the
// resolver returns the smallest unit (Minute, step 1).
func ResolveAutoInterval(min, max time.Time, targetBuckets int) HistogramInterval {
	if targetBuckets <= 0 {
		targetBuckets = 50
	}
	d := max.Sub(min)
	if d < 0 {
		d = -d
	}
	if d == 0 {
		return HistogramInterval{Unit: HistogramIntervalMinute, Step: 1}
	}

	type rung struct {
		interval HistogramInterval
		width    time.Duration
	}
	ladder := []rung{
		{HistogramInterval{Unit: HistogramIntervalMinute, Step: 1}, time.Minute},
		{HistogramInterval{Unit: HistogramIntervalMinute, Step: 5}, 5 * time.Minute},
		{HistogramInterval{Unit: HistogramIntervalMinute, Step: 10}, 10 * time.Minute},
		{HistogramInterval{Unit: HistogramIntervalMinute, Step: 30}, 30 * time.Minute},
		{HistogramInterval{Unit: HistogramIntervalHour, Step: 1}, time.Hour},
		{HistogramInterval{Unit: HistogramIntervalHour, Step: 3}, 3 * time.Hour},
		{HistogramInterval{Unit: HistogramIntervalHour, Step: 12}, 12 * time.Hour},
		{HistogramInterval{Unit: HistogramIntervalDay, Step: 1}, 24 * time.Hour},
		{HistogramInterval{Unit: HistogramIntervalWeek, Step: 1}, 7 * 24 * time.Hour},
		{HistogramInterval{Unit: HistogramIntervalMonth, Step: 1}, 30 * 24 * time.Hour},
	}
	target := time.Duration(targetBuckets)
	for _, r := range ladder {
		if d/r.width <= target {
			return r.interval
		}
	}
	return ladder[len(ladder)-1].interval
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
