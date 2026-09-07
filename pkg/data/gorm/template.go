package gorm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/activatedio/datainfra/pkg/data"
	"github.com/activatedio/datainfra/pkg/reflect"
)

// DefaultPageSize is the page size DoList uses when the caller supplies no
// PageParams, or supplies one whose Count is zero or negative. Every list is
// a page: a caller that wants more than one page follows NextPageToken, or
// uses data.Collect to do so under a ceiling.
const DefaultPageSize = 100

// UnpagedOverflowError is returned by DoList when a template configured
// without KeyColumns/KeyAccessor matches more rows than the requested page
// can hold. Such a template cannot produce a NextPageToken, so returning
// the first Limit rows would silently truncate the result. The remedy is to
// configure KeyColumns and KeyAccessor on the template (the generator emits
// them for every keyed entity) or to narrow the criteria.
type UnpagedOverflowError struct {
	Table string
	Limit int
}

func (e UnpagedOverflowError) Error() string {
	return fmt.Sprintf("table %q matched more than %d rows but has no key columns to page over; "+
		"configure KeyColumns and KeyAccessor on the template or narrow the criteria", e.Table, e.Limit)
}

// MappingTemplate defines operations for mapping between external and internal representations of entities.
type MappingTemplate[E any, I any] interface {
	// GetTable returns the name of the database table associated with the template.
	GetTable() string
	// ApplyContextScopeQueryBuilder applies context-based query modifications to the database query.
	ApplyContextScopeQueryBuilder(ctx context.Context, db *gorm.DB, fetchType data.FetchType) *gorm.DB

	// ApplyContextScopeQueryBuilderForTable is ApplyContextScopeQueryBuilder
	// against an explicitly named table rather than the entity's own.
	//
	// It exists for association queries. A ListBy<Associated> joins an edge
	// table and filters it by the associated key, but the context scope was
	// only ever applied to the entity's table — so the join matched edge rows
	// from *any* scope and paired them with in-scope entity rows. Where the
	// edge table carries the scope column (it is normally part of its primary
	// key), applying the scope to it as well is what makes the association
	// listing mean "associated *here*".
	//
	// The ContextScopeFactory already takes the table name and qualifies its
	// predicate with it, so this needs no new information — only the chance to
	// pass a different table.
	ApplyContextScopeQueryBuilderForTable(ctx context.Context, db *gorm.DB, table string, fetchType data.FetchType) *gorm.DB
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
	keyColumns   []string
	keyAccessor  func(I) []any
}

// TemplateParams defines parameters required for creating templates with optional context scope and table name.
//
// KeyColumns and KeyAccessor enable cursor pagination in DoList. KeyColumns
// lists the snake_case database columns ordered on (in canonical order, one
// entry for single-key entities, multiple for composite keys); KeyAccessor
// returns the values of those columns from a fetched row, in matching
// order. Both must be supplied to enable pagination. Without them DoList
// still limits to one page but cannot hand out a NextPageToken, so it
// returns UnpagedOverflowError rather than truncate when more rows match.
type TemplateParams[E any, I any] struct {
	ContextScope ContextScopeFactory
	Table        string
	KeyColumns   []string
	KeyAccessor  func(E) []any
}

// NewTemplate initializes and returns a Template instance for mapping entities with the specified parameters.
func NewTemplate[E any](params TemplateParams[E, E]) Template[E] {

	return NewMappingTemplate[E, E](MappingTemplateParams[E, E]{
		ContextScope: params.ContextScope,
		Table:        params.Table,
		KeyColumns:   params.KeyColumns,
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
// See TemplateParams for the pagination-related fields KeyColumns and
// KeyAccessor; KeyAccessor on a MappingTemplate reads from the internal
// representation I.
type MappingTemplateParams[E any, I any] struct {
	ContextScope ContextScopeFactory
	Table        string
	ToInternal   func(in E) I
	FromInternal func(in I) E
	KeyColumns   []string
	KeyAccessor  func(I) []any
}

// NewMappingTemplate initializes and returns a new MappingTemplate using the provided MappingTemplateParams.
func NewMappingTemplate[E any, I any](params MappingTemplateParams[E, I]) MappingTemplate[E, I] {

	return &templateImpl[E, I]{
		contextScope: params.ContextScope,
		table:        params.Table,
		toInternal:   params.ToInternal,
		fromInternal: params.FromInternal,
		keyColumns:   params.KeyColumns,
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

// ApplyContextScopeQueryBuilderForTable applies the context scope to table.
// A no-op when the entity is unscoped, which is what keeps association
// listings on unscoped entities unchanged.
func (c *templateImpl[E, I]) ApplyContextScopeQueryBuilderForTable(ctx context.Context, db *gorm.DB,
	table string, fetchType data.FetchType) *gorm.DB {

	if c.contextScope == nil {
		return db
	}
	return db.Scopes(c.contextScope(ctx, table, fetchType).QueryModifier)
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

// DoList retrieves one page of external entities based on the provided
// criteria and list parameters.
//
// Every call returns a page. A nil params.PageParams means the first page of
// DefaultPageSize rows; a Count of zero or less means DefaultPageSize as
// well. When the template was configured with KeyColumns + KeyAccessor,
// DoList paginates with a forward cursor over the key columns: it orders by
// the key ascending, applies PageToken as a strict lower bound, fetches one
// extra row to detect overflow, and populates NextPageToken on the response
// when more rows remain. Tokens are opaque base64 strings; callers should
// treat them as cookies. A result with an empty NextPageToken is complete.
//
// A template without key columns cannot produce a token. It still fetches
// one row past the page; if that row materializes DoList returns
// UnpagedOverflowError instead of a silently truncated list, and a
// PageToken supplied to it is rejected.
//
// params.Selector is applied to the fetched page after the database query,
// so a page may hold fewer rows than Count (even none) while NextPageToken
// is still set. Callers must follow the token, not the row count, to know
// when they are done; data.Collect and data.ExistsAny do this correctly.
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

	count := resolvePageCount(params.PageParams)
	paginate := c.pageable()
	if paginate {
		tx, err = c.applyPageCursor(tx, params.PageParams)
		if err != nil {
			return nil, err
		}
		tx = tx.Order(strings.Join(c.keyColumns, ", "))
	} else if params.PageParams != nil && params.PageParams.PageToken != "" {
		return nil, fmt.Errorf("invalid page token: table %q has no key columns to page over", c.table)
	}
	// Always fetch one row past the page so overflow is detected rather than
	// silently dropped.
	tx = tx.Limit(count + 1)

	if tx.Error != nil {
		return nil, tx.Error
	}

	var results []I
	tx.Find(&results)

	if tx.Error != nil {
		return nil, tx.Error
	}

	var nextToken string
	if len(results) > count {
		if !paginate {
			return nil, UnpagedOverflowError{Table: c.table, Limit: count}
		}
		nextToken, err = c.encodeCursor(c.keyAccessor(results[count-1]))
		if err != nil {
			return nil, err
		}
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

// resolvePageCount returns the page size to apply: PageParams.Count when
// positive, DefaultPageSize otherwise (including a nil PageParams).
func resolvePageCount(pp *data.PageParams) int {
	if pp == nil || pp.Count <= 0 {
		return DefaultPageSize
	}
	return pp.Count
}

// pageable reports whether the template can hand out page tokens, which
// requires both non-empty KeyColumns and a KeyAccessor.
func (c *templateImpl[E, I]) pageable() bool {
	return len(c.keyColumns) > 0 && c.keyAccessor != nil
}

// applyPageCursor applies the PageToken (if any) as a strict lower-bound
// row-constructor WHERE clause: (col1, col2, ...) > (v1, v2, ...). Single-
// key entities reduce to (col1) > (v1) which the SQL engines accept.
func (c *templateImpl[E, I]) applyPageCursor(tx *gorm.DB, pp *data.PageParams) (*gorm.DB, error) {
	if pp == nil || pp.PageToken == "" {
		return tx, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(pp.PageToken)
	if err != nil {
		return nil, fmt.Errorf("invalid page token: %w", err)
	}
	var values []string
	if err := json.Unmarshal(decoded, &values); err != nil {
		return nil, fmt.Errorf("invalid page token: %w", err)
	}
	if len(values) != len(c.keyColumns) {
		return nil, fmt.Errorf("invalid page token: expected %d values, got %d", len(c.keyColumns), len(values))
	}

	cols := "(" + strings.Join(c.keyColumns, ", ") + ")"
	placeholders := "(" + strings.Repeat("?, ", len(values))
	placeholders = strings.TrimSuffix(placeholders, ", ") + ")"
	args := make([]any, len(values))
	for i, v := range values {
		args[i] = v
	}
	return tx.Where(fmt.Sprintf("%s > %s", cols, placeholders), args...), nil
}

func (c *templateImpl[E, I]) encodeCursor(values []any) (string, error) {
	strs := make([]string, len(values))
	for i, v := range values {
		strs[i] = fmt.Sprint(v)
	}
	encoded, err := json.Marshal(strs)
	if err != nil {
		return "", fmt.Errorf("encode page token: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encoded), nil
}
