package gorm

import (
	"context"
	"fmt"

	"github.com/activatedio/datainfra/pkg/data"
)

type filterKeysTemplateImpl[E any, I any, K comparable] struct {
	template   MappingTemplate[E, I]
	findColumn string
}

// MappingFilterKeysTemplateImplOptions defines options for configuring a filter keys template implementation.
// It includes a mapping template and the column used to find entities.
type MappingFilterKeysTemplateImplOptions[E any, I any, K comparable] struct {
	Template   MappingTemplate[E, I]
	FindColumn string
}

// NewMappingFilterKeysTemplate creates a new filter keys template implementation for managing entity key filtering.
// It uses the provided options, including a mapping template and a column to identify entities.
func NewMappingFilterKeysTemplate[E any, I any, K comparable](options MappingFilterKeysTemplateImplOptions[E, I, K]) data.FilterKeysTemplate[K] {
	return &filterKeysTemplateImpl[E, I, K]{
		template:   options.Template,
		findColumn: options.FindColumn,
	}
}

// FilterKeys retrieves and filters a subset of input keys from the database based on the configured column and context.
func (c *filterKeysTemplateImpl[E, I, K]) FilterKeys(ctx context.Context, keys []K) ([]K, error) {

	// TODO - we may want to move this into the Template
	tx := GetDB(ctx).Table(c.template.GetTable())
	tx = c.template.ApplyContextScopeQueryBuilder(ctx, tx, data.FetchTypeKeys)

	tx.Select(c.findColumn).Where(fmt.Sprintf("%s IN ?", c.findColumn), keys)

	var result []K

	tx.Find(&result)

	if tx.Error != nil {
		return nil, tx.Error
	}

	return result, nil

}
