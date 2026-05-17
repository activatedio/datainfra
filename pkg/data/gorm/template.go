package gorm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/activatedio/datainfra/pkg/data"
	"github.com/activatedio/datainfra/pkg/reflect"
)

// DefaultPageSize is the page size used when ListParams.PageParams is
// non-nil but PageParams.Count is zero or negative.
const DefaultPageSize = 100

// MappingTemplate defines operations for mapping between external and internal representations of entities.
type MappingTemplate[E any, I any] interface {
	// GetTable returns the name of the database table associated with the template.
	GetTable() string
	// ApplyContextScopeQueryBuilder applies context-based query modifications to the database query.
	ApplyContextScopeQueryBuilder(ctx context.Context, db *gorm.DB, fetchType data.FetchType) *gorm.DB
	// ApplyContextScopeValueInjector applies context-based value injections to the provided internal entity.
	ApplyContextScopeValueInjector(ctx context.Context, entry I, fetchType data.FetchType)
	// DoFind performs a database query based on a delegate and returns a single mapped external entity or an error.
	DoFind(ctx context.Context, delegate func(db *gorm.DB, entry I) (*gorm.DB, error)) (E, error)
	// DoList executes a query based on criteria and parameters, returning a paginated list of external entities or an error.
	DoList(ctx context.Context, criteriaBuilder func(tx *gorm.DB) *gorm.DB, params data.ListParams) (*data.List[E], error)
	// ToInternal converts an external entity representation into its internal counterpart.
	ToInternal(in E) I
	// FromInternal converts an internal entity representation back into its external form.
	FromInternal(in I) E
}

// Template describes an interface for mapping operations specific to entities, extending MappingTemplate with equivalent types.
type Template[E any] interface {
	MappingTemplate[E, E]
}

type templateImpl[E any, I any] struct {
	contextScope ContextScopeFactory
	table        string
	toInternal   func(in E) I
	fromInternal func(in I) E
	keyColumn    string
	keyAccessor  func(I) any
}

// TemplateParams defines parameters required for creating templates with optional context scope and table name.
//
// KeyColumn and KeyAccessor enable cursor pagination in DoList. KeyColumn is
// the snake_case database column ordered on; KeyAccessor returns the value
// of that column on a fetched row. Both must be supplied to enable
// pagination; if either is empty, DoList falls back to a fixed-limit query
// with no page tokens.
type TemplateParams[E any, I any] struct {
	ContextScope ContextScopeFactory
	Table        string
	KeyColumn    string
	KeyAccessor  func(E) any
}

// NewTemplate initializes and returns a Template instance for mapping entities with the specified parameters.
func NewTemplate[E any](params TemplateParams[E, E]) Template[E] {

	return NewMappingTemplate[E, E](MappingTemplateParams[E, E]{
		ContextScope: params.ContextScope,
		Table:        params.Table,
		KeyColumn:    params.KeyColumn,
		KeyAccessor:  params.KeyAccessor,
		ToInternal: func(in E) E {
			return in
		},
		FromInternal: func(in E) E {
			return in
		},
	})

}

// MappingTemplateParams defines parameters for configuring a mapping template, including context, table, and conversion functions.
//
// See TemplateParams for the pagination-related fields KeyColumn and
// KeyAccessor; KeyAccessor on a MappingTemplate reads from the internal
// representation I.
type MappingTemplateParams[E any, I any] struct {
	ContextScope ContextScopeFactory
	Table        string
	ToInternal   func(in E) I
	FromInternal func(in I) E
	KeyColumn    string
	KeyAccessor  func(I) any
}

// NewMappingTemplate initializes and returns a new MappingTemplate using the provided MappingTemplateParams.
func NewMappingTemplate[E any, I any](params MappingTemplateParams[E, I]) MappingTemplate[E, I] {

	return &templateImpl[E, I]{
		contextScope: params.ContextScope,
		table:        params.Table,
		toInternal:   params.ToInternal,
		fromInternal: params.FromInternal,
		keyColumn:    params.KeyColumn,
		keyAccessor:  params.KeyAccessor,
	}
}

// ToInternal converts an instance of type E to its internal representation of type I using the toInternal function.
func (c *templateImpl[E, I]) ToInternal(in E) I {
	return c.toInternal(in)
}

// FromInternal converts an internal representation of type I to an external representation of type E using the fromInternal function.
func (c *templateImpl[E, I]) FromInternal(in I) E {
	return c.fromInternal(in)
}

// GetTable retrieves the table name managed by the templateImpl instance.
func (c *templateImpl[E, I]) GetTable() string {
	return c.table
}

// ApplyContextScopeQueryBuilder applies context-specific query scopes to the provided Gorm DB instance based on fetch type.
func (c *templateImpl[E, I]) ApplyContextScopeQueryBuilder(ctx context.Context, db *gorm.DB, fetchType data.FetchType) *gorm.DB {

	var scopes []func(*gorm.DB) *gorm.DB
	if c.contextScope != nil {
		scopes = append(scopes, c.contextScope(ctx, c.table, fetchType).QueryModifier)
	}
	return db.Scopes(scopes...)
}

// ApplyContextScopeValueInjector injects context-specific values into the provided entry based on fetch type and scope configuration.
func (c *templateImpl[E, I]) ApplyContextScopeValueInjector(ctx context.Context, entry I, fetchType data.FetchType) {

	if c.contextScope != nil {
		c.contextScope(ctx, c.table, fetchType).ValueInjector(entry)
	}
}

// DoFind performs a database query using the provided delegate function and processes the result based on row count.
func (c *templateImpl[E, I]) DoFind(ctx context.Context, delegate func(db *gorm.DB, entry I) (*gorm.DB, error)) (E, error) {

	db, err := GetDB(ctx)
	if err != nil {
		return reflect.NilInterface[E](), err
	}
	tx := db.Table(c.table)
	tx = c.ApplyContextScopeQueryBuilder(ctx, tx, data.FetchTypeDetail)

	e := reflect.ZeroInterface[I]()

	tx, err = delegate(tx, e)

	if tx.Error != nil {
		err = tx.Error
	}

	switch {
	case err != nil:
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return reflect.NilInterface[E](), nil
		}
		return reflect.NilInterface[E](), err
	case tx.RowsAffected == 0:
		return reflect.NilInterface[E](), nil
	case tx.RowsAffected == 1:
		return c.fromInternal(e), nil
	default:
		// Rows are more than 1
		return reflect.NilInterface[E](), fmt.Errorf("expected 1 record, but was %d", tx.RowsAffected)
	}
}

// DoList retrieves a list of external entities based on the provided criteria and list parameters.
//
// When params.PageParams is non-nil and the template was configured with a
// KeyColumn + KeyAccessor, DoList paginates with a forward cursor over the
// key column: it orders by the key ascending, applies PageToken as a strict
// lower bound, fetches one extra row to detect overflow, and populates
// NextPageToken on the response when more rows remain. Tokens are opaque
// base64 strings; callers should treat them as cookies.
//
// When pagination is not configured, DoList applies a fixed DefaultPageSize
// limit and never returns a NextPageToken.
func (c *templateImpl[E, I]) DoList(ctx context.Context, //nolint:gocyclo // pagination, scope, criteria, and label filter checks each add a branch
	criteriaBuilder func(tx *gorm.DB) *gorm.DB,
	params data.ListParams) (*data.List[E], error) {

	db, err := GetDB(ctx)
	if err != nil {
		return nil, err
	}
	tx := db.Table(c.table)

	tx = c.ApplyContextScopeQueryBuilder(ctx, tx, data.FetchTypeList)

	if criteriaBuilder != nil {
		tx = criteriaBuilder(tx)
	}

	count, paginate := c.resolvePageCount(params.PageParams)
	if paginate {
		var err error
		tx, err = c.applyPageCursor(tx, params.PageParams)
		if err != nil {
			return nil, err
		}
		tx = tx.Order(c.keyColumn).Limit(count + 1)
	} else {
		tx = tx.Limit(count)
	}

	if tx.Error != nil {
		return nil, tx.Error
	}

	var results []I
	tx.Find(&results)

	if tx.Error != nil {
		return nil, tx.Error
	}

	var nextToken string
	if paginate && len(results) > count {
		nextToken = c.encodeCursor(c.keyAccessor(results[count-1]))
		results = results[:count]
	}

	externalResults := make([]E, len(results))
	for i, in := range results {
		externalResults[i] = c.fromInternal(in)
	}

	if params.Selector != nil {
		externalResults, err = data.FilterByLabels(params.Selector, externalResults)
		if err != nil {
			return nil, err
		}
	}

	return &data.List[E]{
		NextPageToken: nextToken,
		List:          externalResults,
	}, nil
}

// resolvePageCount returns the row limit to apply and whether cursor
// pagination is active. Pagination requires both a configured KeyColumn /
// KeyAccessor and a non-nil PageParams.
func (c *templateImpl[E, I]) resolvePageCount(pp *data.PageParams) (int, bool) {
	if pp == nil || c.keyColumn == "" || c.keyAccessor == nil {
		return DefaultPageSize, false
	}
	if pp.Count <= 0 {
		return DefaultPageSize, true
	}
	return pp.Count, true
}

// applyPageCursor applies the PageToken (if any) as a strict lower-bound
// WHERE clause on the key column.
func (c *templateImpl[E, I]) applyPageCursor(tx *gorm.DB, pp *data.PageParams) (*gorm.DB, error) {
	if pp.PageToken == "" {
		return tx, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(pp.PageToken)
	if err != nil {
		return nil, fmt.Errorf("invalid page token: %w", err)
	}
	return tx.Where(fmt.Sprintf("%s > ?", c.keyColumn), string(decoded)), nil
}

func (c *templateImpl[E, I]) encodeCursor(key any) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(key)))
}
