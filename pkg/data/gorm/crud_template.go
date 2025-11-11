package gorm

import (
	"context"
	"fmt"

	"github.com/activatedio/datainfra/pkg/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FindBuilder defines a function type for building queries to find entities in the database based on a given key.
type FindBuilder[K comparable] func(ctx context.Context, tx *gorm.DB, key K) *gorm.DB

type crudTemplateImpl[E data.Entity[K], I any, K comparable] struct {
	template    MappingTemplate[E, I]
	findBuilder FindBuilder[K]
}

// MappingCrudTemplateImplOptions provides configuration options for creating a mapping-based CRUD template implementation.
// E represents the external entity type, I represents the internal entity type, and K is the type of the entity's key.
type MappingCrudTemplateImplOptions[E data.Entity[K], I any, K comparable] struct {
	// Template specifies the mapping template used for conversions between external and internal representations.
	Template MappingTemplate[E, I]
	// FindBuilder is an optional query builder function for locating entities, defaulting to queries based on "id".
	FindBuilder FindBuilder[K]
}

// NewMappingCrudTemplate creates a generic CRUD template using a mapping template and optional find builder configuration.
func NewMappingCrudTemplate[E data.Entity[K], I any, K comparable](options MappingCrudTemplateImplOptions[E, I, K]) data.CrudTemplate[E, K] {
	return &crudTemplateImpl[E, I, K]{
		template:    options.Template,
		findBuilder: options.FindBuilder,
	}
}

// CrudTemplateImplOptions provides configuration for creating a CRUD template implementation for entity operations.
// E represents the external entity type that must implement the model.Entity interface.
// K represents the entity's key type, which must be a comparable type.
type CrudTemplateImplOptions[E data.Entity[K], K comparable] struct {
	Template Template[E]
	// Optional column to use for the find method. Defaults to "id"
	FindBuilder FindBuilder[K]
}

// NewCrudTemplate creates a CRUD template for managing entities of type E with a key of type K using specified options.
func NewCrudTemplate[E data.Entity[K], K comparable](options CrudTemplateImplOptions[E, K]) data.CrudTemplate[E, K] {
	return NewMappingCrudTemplate[E, E, K](MappingCrudTemplateImplOptions[E, E, K]{
		Template:    options.Template,
		FindBuilder: options.FindBuilder,
	})
}

// Find retrieves a single entity of type E based on the criteria defined by the criteriaBuilder function.
func (c *crudTemplateImpl[E, I, K]) Find(ctx context.Context, criteriaBuilder func(tx *gorm.DB) *gorm.DB) (E, error) {
	return c.template.DoFind(ctx, func(db *gorm.DB, entry I) (*gorm.DB, error) {
		if criteriaBuilder != nil {
			db = criteriaBuilder(db)
		}
		return db.First(entry), nil
	})
}

// FindByKey retrieves a single entity of type E using the provided key K and the defined findBuilder logic.
func (c *crudTemplateImpl[E, I, K]) FindByKey(ctx context.Context, key K) (E, error) {
	return c.template.DoFind(ctx, func(db *gorm.DB, entry I) (*gorm.DB, error) {
		return c.findBuilder(ctx, db, key).Find(entry), nil
	})
}

// ExistsByKey checks if an entity with the specified key exists in the database and returns a boolean result with an error.
func (c *crudTemplateImpl[E, I, K]) ExistsByKey(ctx context.Context, key K) (bool, error) {

	var got any
	var err error

	got, err = c.FindByKey(ctx, key)

	return got != nil, err
}

// List retrieves a paginated list of entities of type E based on the provided filter and pagination parameters.
func (c *crudTemplateImpl[E, I, K]) List(ctx context.Context, _ *E, params data.ListParams) (*data.List[E], error) {
	return c.template.DoList(ctx, nil, params)
}

// ListAll retrieves all entities of type E based on the provided list parameters without any specific criteria.
func (c *crudTemplateImpl[E, I, K]) ListAll(ctx context.Context, params data.ListParams) (*data.List[E], error) {
	return c.template.DoList(ctx, nil, params)
}

// Create inserts a new entity into the database, ignoring conflicts if the entity already exists and returns an error if any occur.
func (c *crudTemplateImpl[E, I, K]) Create(ctx context.Context, entity E) error {
	// TODO - Add validation call

	internal := c.template.ToInternal(entity)
	c.template.ApplyContextScopeValueInjector(ctx, internal, data.FetchTypeNone)

	tx := GetDB(ctx).Table(c.template.GetTable()).Clauses(clause.OnConflict{DoNothing: true}).Create(internal)

	switch {
	case tx.Error != nil:
		return tx.Error
	case tx.RowsAffected == 0:
		return data.EntityAlreadyExists{}
	default:
		return nil
	}
}

// Update modifies an existing entity in the database and returns an error if the operation fails.
func (c *crudTemplateImpl[E, I, K]) Update(ctx context.Context, entity E) error {

	internal := c.template.ToInternal(entity)
	c.template.ApplyContextScopeValueInjector(ctx, internal, data.FetchTypeNone)

	return GetDB(ctx).Table(c.template.GetTable()).Save(internal).Error
}

// Delete removes an entity of type E from the database based on the provided key K, using the findBuilder logic.
func (c *crudTemplateImpl[E, I, K]) Delete(ctx context.Context, key K) error {
	db := GetDB(ctx).Table(c.template.GetTable())
	return c.findBuilder(ctx, db, key).Delete(new(E)).Error
}

// DeleteEntity removes the provided entity of type E from the database and returns an error if the operation fails.
func (c *crudTemplateImpl[E, I, K]) DeleteEntity(ctx context.Context, entity E) error {
	// TODO - need to validate access
	// TODO - unit test
	return GetDB(ctx).Table(c.template.GetTable()).Delete(entity).Error
}

// SingleFindBuilder returns a FindBuilder function that constructs a query to find an entity based on the specified column.
func SingleFindBuilder[K comparable](findColumn string) FindBuilder[K] {
	return func(_ context.Context, tx *gorm.DB, key K) *gorm.DB {
		return tx.Where(fmt.Sprintf("%s = ?", findColumn), key)
	}
}
