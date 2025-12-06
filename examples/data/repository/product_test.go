package repository_test

import (
	"testing"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/examples/data/repository"
	"github.com/activatedio/datainfra/pkg/data"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductRepository_Crud(t *testing.T) {
	a := assert.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.ProductRepository) {
		datatesting.DoTestCrudRepository[*model.Product, string](t, cp.GetContext(), unit,
			&datatesting.CrudTestFixture[*model.Product, string]{
				KeyExists:  "1",
				KeyMissing: "invalid",
				NewEntity: func() *model.Product {
					return &model.Product{}
				},
				ExtractKey: func(e *model.Product) string {
					return e.SKU
				},
				AssertDetailEntry: func(_ *testing.T, e *model.Product) {
					a.NotEmpty(e.SKU)
					a.NotEmpty(e.Description)
				},
				ModifyBeforeCreate: func(e *model.Product) {
					e.SKU = uuid.New().String()
					e.Description = "initial"
				},
				AssertAfterCreate: func(_ *testing.T, e *model.Product) {
					a.Equal("initial", e.Description)
				},
				ModifyBeforeUpdate: func(e *model.Product) {
					e.Description = "modified"
				},
				AssertAfterUpdate: func(_ *testing.T, e *model.Product) {
					a.Equal("modified", e.Description)
				},
			})
	})
}

func TestProductRepository_ListByProduct(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.ProductRepository) {

		ctx := cp.GetContext()

		got, err := unit.ListByCategory(ctx, "a", data.ListParams{})
		r.NoError(err)
		a.Len(got.List, 2)
	})
}

func TestProductRepository_Associate(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider,
		unit repository.ProductRepository,
		cr repository.CategoryRepository,
	) {

		ctx := cp.GetContext()

		skus := []string{
			uuid.New().String(),
			uuid.New().String(),
			uuid.New().String(),
			uuid.New().String(),
		}

		names := []string{
			uuid.New().String(),
			uuid.New().String(),
			uuid.New().String(),
		}

		for _, s := range skus {
			r.NoError(unit.Create(ctx, &model.Product{SKU: s, Description: s}))
		}
		for _, n := range names {
			r.NoError(cr.Create(ctx, &model.Category{Name: n, Description: n}))
		}

		got, err := unit.ListByCategory(ctx, names[0], data.ListParams{})
		r.NoError(err)
		a.Empty(got.List)

		for _, s := range skus[:2] {
			r.NoError(unit.AssociateCategories(ctx, s, names[:2], nil))
		}

		for _, n := range names[:2] {
			got, err = unit.ListByCategory(ctx, n, data.ListParams{})
			r.NoError(err)
			a.Len(got.List, 2)
		}

		for _, s := range skus[:2] {
			r.NoError(unit.AssociateCategories(ctx, s, names[2:3], names[1:2]))
		}

		for _, n := range []string{names[0], names[2]} {
			got, err = unit.ListByCategory(ctx, n, data.ListParams{})
			r.NoError(err)
			a.Len(got.List, 2)
		}

		for _, s := range skus {
			r.NoError(unit.AssociateCategories(ctx, s, nil, names))
		}

		for _, n := range names {
			got, err = unit.ListByCategory(ctx, n, data.ListParams{})
			r.NoError(err)
			a.Empty(got.List)
		}

	})
}
