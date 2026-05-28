package gorm

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/activatedio/datainfra/pkg/data"
)

// PredicateBinder translates a single SearchPredicate into a gorm WHERE
// clause. Binders are paired with descriptors via SearchPredicateBinding and
// invoked by the generated Search method, allowing the SQL fragment to be
// chosen per predicate name (and per database dialect).
type PredicateBinder func(tx *gorm.DB, p *data.SearchPredicate) (*gorm.DB, error)

// SearchPredicateBinding pairs a search predicate descriptor (exposed
// through GetSearchPredicates) with the gorm binder that knows how to
// translate that predicate into a WHERE clause.
type SearchPredicateBinding struct {
	Descriptor *data.SearchPredicateDescriptor
	Binder     PredicateBinder
}

// PostgresKeywordsBinder returns a PredicateBinder that filters using
// PostgreSQL full-text search on the given column. The predicate operator
// must be SearchOperatorStringMatch; the StringValue is fed through
// plainto_tsquery with the english config.
func PostgresKeywordsBinder(column string) PredicateBinder {
	return func(tx *gorm.DB, p *data.SearchPredicate) (*gorm.DB, error) {
		if p.Operator != data.SearchOperatorStringMatch {
			return nil, fmt.Errorf("PostgresKeywordsBinder: operator %v not supported", p.Operator)
		}
		return tx.Where(fmt.Sprintf("%s @@ plainto_tsquery('english', ?)", column), p.StringValue), nil
	}
}

// LikeBinder returns a PredicateBinder that filters using a case-insensitive
// LIKE on the given column. The predicate operator must be
// SearchOperatorStringMatch. Works across SQL dialects but does not use
// full-text indexes.
func LikeBinder(column string) PredicateBinder {
	return func(tx *gorm.DB, p *data.SearchPredicate) (*gorm.DB, error) {
		if p.Operator != data.SearchOperatorStringMatch {
			return nil, fmt.Errorf("LikeBinder: operator %v not supported", p.Operator)
		}
		pattern := "%" + strings.ToLower(p.StringValue) + "%"
		return tx.Where(fmt.Sprintf("LOWER(%s) LIKE ?", column), pattern), nil
	}
}

// DialectBinder returns a PredicateBinder that dispatches to a per-dialect
// binder using the configured gorm.Dialector name (e.g. "postgres",
// "sqlite"). Returns an error if no binder is registered for the runtime
// dialect.
func DialectBinder(byDialect map[string]PredicateBinder) PredicateBinder {
	return func(tx *gorm.DB, p *data.SearchPredicate) (*gorm.DB, error) {
		name := tx.Name()
		b, ok := byDialect[name]
		if !ok {
			return nil, fmt.Errorf("DialectBinder: no binder for dialect %q", name)
		}
		return b(tx, p)
	}
}

type searchTemplateImpl[E any, I any] struct {
	template    MappingTemplate[E, I]
	bindings    []SearchPredicateBinding
	bindingsMap map[string]SearchPredicateBinding
	descriptors []*data.SearchPredicateDescriptor
}

// GetSearchPredicates returns the descriptors for the predicates the
// template was configured with, in registration order.
func (c *searchTemplateImpl[E, I]) GetSearchPredicates(_ context.Context) ([]*data.SearchPredicateDescriptor, error) {
	return c.descriptors, nil
}

// GetSearchPredicateDescriptor looks up a registered descriptor by name and
// returns ErrUnknownSearchPredicate if no binding matches.
func (c *searchTemplateImpl[E, I]) GetSearchPredicateDescriptor(_ context.Context, name string) (*data.SearchPredicateDescriptor, error) {
	b, ok := c.bindingsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownSearchPredicate, name)
	}
	return b.Descriptor, nil
}

// ErrUnknownSearchPredicate is returned when a Search criterion references a
// predicate name that is not registered with the template.
var ErrUnknownSearchPredicate = errors.New("unknown search predicate")

// MappingSearchTemplateParams defines parameters required to create a mapping search template.
// Type E represents the external entity, and type I represents the internal entity.
type MappingSearchTemplateParams[E any, I any] struct {
	Template MappingTemplate[E, I]
	Bindings []SearchPredicateBinding
}

// NewMappingSearchTemplate creates a new search template with specified mapping and bindings.
// Type E represents the external entity, and type I represents the internal entity.
// It returns a data.SearchTemplate for executing search operations.
func NewMappingSearchTemplate[E any, I any](params MappingSearchTemplateParams[E, I]) data.SearchTemplate[E] {

	bindingsMap := make(map[string]SearchPredicateBinding, len(params.Bindings))
	descriptors := make([]*data.SearchPredicateDescriptor, 0, len(params.Bindings))
	for _, b := range params.Bindings {
		bindingsMap[b.Descriptor.Name] = b
		descriptors = append(descriptors, b.Descriptor)
	}

	return &searchTemplateImpl[E, I]{
		template:    params.Template,
		bindings:    params.Bindings,
		bindingsMap: bindingsMap,
		descriptors: descriptors,
	}
}

// SearchTemplateParams represents the parameters for creating a search template.
// Type E refers to the external entity associated with the template.
type SearchTemplateParams[E any] struct {
	Template Template[E]
	Bindings []SearchPredicateBinding
}

// Search applies every supplied predicate as an AND-combined WHERE clause
// using the registered binder for each predicate name. An empty criteria
// slice is treated as "no filter" — the query degenerates to a paginated
// list of every row, matching the natural REST semantics of a search
// request with no constraints. Returns an error only when a predicate
// name is not registered.
func (c *searchTemplateImpl[E, I]) Search(ctx context.Context, criteria []*data.SearchPredicate, pageParams *data.PageParams) (*data.List[*data.SearchResult[E]], error) {

	got, err := c.template.DoList(ctx, func(tx *gorm.DB) *gorm.DB {
		for _, p := range criteria {
			b, ok := c.bindingsMap[p.Name]
			if !ok {
				_ = tx.AddError(fmt.Errorf("%w: %q", ErrUnknownSearchPredicate, p.Name))
				return tx
			}
			next, bindErr := b.Binder(tx, p)
			if bindErr != nil {
				_ = tx.AddError(bindErr)
				return tx
			}
			tx = next
		}
		return tx
	}, data.ListParams{
		PageParams: pageParams,
	})

	if err != nil {
		return nil, err
	}

	result := make([]*data.SearchResult[E], len(got.List))
	for i, e := range got.List {
		result[i] = &data.SearchResult[E]{
			Score:  0,
			Entity: e,
		}
	}

	return &data.List[*data.SearchResult[E]]{
		NextPageToken: got.NextPageToken,
		List:          result,
	}, nil
}
