package gorm

import (
	"context"

	"github.com/activatedio/datainfra/pkg/data"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type searchTemplateImpl[E any, I any] struct {
	template         MappingTemplate[E, I]
	searchPredicates []*data.SearchPredicateDescriptor
}

// GetSearchPredicates retrieves a list of search predicate descriptors defined for the current search template.
func (c *searchTemplateImpl[E, I]) GetSearchPredicates(_ context.Context) ([]*data.SearchPredicateDescriptor, error) {
	// TODO - this is begging for some constants
	return c.searchPredicates, nil
}

// GetSearchPredicateDescriptor retrieves a descriptor for a search predicate by name, supporting only "keywords".
func (c *searchTemplateImpl[E, I]) GetSearchPredicateDescriptor(_ context.Context, name string) (*data.SearchPredicateDescriptor, error) {
	if name != "keywords" {
		return nil, errors.New("only keywords supported")
	}
	return &data.SearchPredicateDescriptor{
		Name:      "keywords",
		Label:     "Keywords",
		Operators: []data.SearchOperator{data.SearchOperatorStringMatch},
	}, nil
}

// MappingSearchTemplateParams defines parameters required to create a mapping search template.
// Type E represents the external entity, and type I represents the internal entity.
type MappingSearchTemplateParams[E any, I any] struct {
	Template         MappingTemplate[E, I]
	SearchPredicates []*data.SearchPredicateDescriptor
}

// NewMappingSearchTemplate creates a new search template with specified mapping and search predicates.
// Type E represents the external entity, and type I represents the internal entity.
// It returns a repository.SearchTemplate for executing search operations based on the given parameters.
func NewMappingSearchTemplate[E any, I any](params MappingSearchTemplateParams[E, I]) data.SearchTemplate[E] {
	return &searchTemplateImpl[E, I]{
		template:         params.Template,
		searchPredicates: params.SearchPredicates,
	}
}

// SearchTemplateParams represents the parameters for creating a search template.
// Type E refers to the external entity associated with the template.
type SearchTemplateParams[E any] struct {
	Template         Template[E]
	SearchPredicates []*data.SearchPredicateDescriptor
}

// Search performs a search based on the specified criteria and pagination parameters, returning a list of search results.
func (c *searchTemplateImpl[E, I]) Search(ctx context.Context, criteria []*data.SearchPredicate, pageParams *data.PageParams) (*data.List[*data.SearchResult[E]], error) {

	// Right now we only support a single predicate of name "keywords"
	if len(criteria) != 1 || criteria[0].Name != "keywords" || criteria[0].Operator != data.SearchOperatorStringMatch {
		return nil, errors.New("only single predicate of keywords supported")
	}

	keywords := criteria[0].StringValue

	got, err := c.template.DoList(ctx, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("full_text @@ plainto_tsquery('english', ?)", keywords)
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
		List: result,
	}, nil

}
