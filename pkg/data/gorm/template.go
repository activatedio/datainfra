package gorm

import (
	"context"
	"errors"
	"fmt"

	"github.com/activatedio/datainfra/pkg/data"
	"github.com/activatedio/datainfra/pkg/reflect"
	"gorm.io/gorm"
)

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
}

// TemplateParams defines parameters required for creating templates with optional context scope and table name.
type TemplateParams[E any, I any] struct {
	CustomContextScope ContextScopeFactory
	Table              string
}

// NewTemplate initializes and returns a Template instance for mapping entities with the specified parameters.
func NewTemplate[E any](params TemplateParams[E, E]) Template[E] {

	return NewMappingTemplate[E, E](MappingTemplateParams[E, E]{
		CustomContextScope: params.CustomContextScope,
		Table:              params.Table,
		ToInternal: func(in E) E {
			return in
		},
		FromInternal: func(in E) E {
			return in
		},
	})

}

// MappingTemplateParams defines parameters for configuring a mapping template, including context, table, and conversion functions.
type MappingTemplateParams[E any, I any] struct {
	CustomContextScope ContextScopeFactory
	Table              string
	ToInternal         func(in E) I
	FromInternal       func(in I) E
}

// NewMappingTemplate initializes and returns a new MappingTemplate using the provided MappingTemplateParams.
func NewMappingTemplate[E any, I any](params MappingTemplateParams[E, I]) MappingTemplate[E, I] {

	var contextScope ContextScopeFactory

	/*

		TODO - need to figure out the context scope factory

		if params.CustomContextScope != nil {
			contextScope = params.CustomContextScope
		} else {
			contextScope = standardContextScope[E]()
		}

	*/

	return &templateImpl[E, I]{
		contextScope: contextScope,
		table:        params.Table,
		toInternal:   params.ToInternal,
		fromInternal: params.FromInternal,
	}
}

/*
TODO - need to handle this context scope
func standardContextScope[E any]() ContextScopeFactory {

	switch model.ScopedFor[E]() {
	case model.TenantScoped:
		return WithContextTenantScope
	case model.IssuerScoped:
		return WithContextIssuerScope
	case model.RealmScoped:
		return WithContextRealmScope
	case model.RealmViaProviderScoped:
		return WithContextRealmViaProviderScope
	case model.AudienceScoped:
		return WithContextAudienceScope
	default:
		return nil
	}
}
*/

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

	tx := GetDB(ctx).Table(c.table)
	tx = c.ApplyContextScopeQueryBuilder(ctx, tx, data.FetchTypeDetail)

	e := reflect.ZeroInterface[I]()

	tx, err := delegate(tx, e)

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
func (c *templateImpl[E, I]) DoList(ctx context.Context,
	criteriaBuilder func(tx *gorm.DB) *gorm.DB,
	params data.ListParams) (*data.List[E], error) {

	tx := GetDB(ctx).Table(c.table)

	tx = c.ApplyContextScopeQueryBuilder(ctx, tx, data.FetchTypeList)

	if criteriaBuilder == nil {
		criteriaBuilder = func(tx *gorm.DB) *gorm.DB {
			return tx
		}
	}

	/*

			// TODO - implement

			if pageParams == nil {
				pageParams = &repository_inner.PageParams{
					First: "",
					Last:  "",
					Count: 0,
				}
			}


		if pageParams.Count == 0 {
			// We default to a sensible number
			pageParams.Count = 100
		}



			// TODO - fix

			for _, sc := range sortCriteria {
				f := ToSnakeCase(sc.Field)
				s := f
				if sc.Reverse != (pageParams.First != "") {
					s = s + " DESC"
				}
				tx = tx.Order(s)
				if pageParams.First != "" {
					tx = tx.Where(f+" < ?", pageParams.First)
				} else if pageParams.Last != "" {
					tx = tx.Where(f+" > ?", pageParams.Last)
				}
			}

	*/

	// tx = tx.Limit(pageParams.Count + 1)
	tx = tx.Limit(100)

	tx = criteriaBuilder(tx)

	if tx.Error != nil {
		return nil, tx.Error
	}

	var results []I

	tx.Find(&results)

	if tx.Error != nil {
		return nil, tx.Error
	}

	/*
		TODO - Fix
		overflow := pageParams.Count != 0 && len(results) == pageParams.Count+1

		pageInfo := &repository_inner.PageInfo{}

		if pageParams.First != "" {
			reverseAny(results)
			pageInfo.HasNextPage = true
			if overflow {
				pageInfo.HasPreviousPage = true
			}
		} else if pageParams.Last != "" {
			pageInfo.HasPreviousPage = true
			if overflow {
				pageInfo.HasNextPage = true
			}
		} else {
			if overflow {
				pageInfo.HasNextPage = true
			}
		}

		if overflow {
			if pageParams.First != "" {
				results = results[1:]
			} else {
				results = results[:len(results)-1]
			}
		}

		if len(results) > 0 {
			pageInfo.StartCursor = makeCursor2(sortCriteria, results[0])
			pageInfo.EndCursor = makeCursor2(sortCriteria, results[len(results)-1])
		}
	*/

	externalResults := make([]E, len(results))

	for i, in := range results {
		externalResults[i] = c.fromInternal(in)
	}

	if params.Selector != nil {
		var err error
		externalResults, err = data.FilterByLabels(params.Selector, externalResults)
		if err != nil {
			return nil, err
		}
	}

	return &data.List[E]{
		List: externalResults,
	}, nil
}
