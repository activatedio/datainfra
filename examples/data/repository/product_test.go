package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/examples/data/repository"
	"github.com/activatedio/datainfra/pkg/data"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
)

func TestProductRepository_Search(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	datatesting.Run(t, AppFixtures, func(md *ProfileMetadata, cp datatesting.ContextProvider, unit repository.ProductRepository) {
		datatesting.DoTestSearch[*model.Product, repository.ProductRepository](t, cp.GetContext(), unit,
			&datatesting.SearchTestFixture[*model.Product, repository.ProductRepository]{
				FixtureEntries: func() map[string]*datatesting.SearchTestFixtureEntry[*model.Product] {
					switch md.Name {
					case "sqlite":
						return map[string]*datatesting.SearchTestFixtureEntry[*model.Product]{
							"@keywords-via-dialect-binder": {
								Arrange: func(ctx context.Context) (context.Context, []*data.SearchPredicate) {
									return ctx, []*data.SearchPredicate{
										{
											Name:        "@keywords",
											Operator:    data.SearchOperatorStringMatch,
											StringValue: "Test",
										},
									}
								},
								Assert: func(got *data.List[*data.SearchResult[*model.Product]], err error) {
									r.NoError(err)
									a.Len(got.List, 2)
								},
							},
							"empty-criteria-lists-all": {
								Arrange: func(ctx context.Context) (context.Context, []*data.SearchPredicate) {
									return ctx, nil
								},
								Assert: func(got *data.List[*data.SearchResult[*model.Product]], err error) {
									r.NoError(err)
									a.Len(got.List, 4)
								},
							},
						}
					case "postgres":
						return map[string]*datatesting.SearchTestFixtureEntry[*model.Product]{
							"@keywords": {
								Arrange: func(ctx context.Context) (context.Context, []*data.SearchPredicate) {
									return ctx, []*data.SearchPredicate{
										{
											Name:        "@keywords",
											Operator:    data.SearchOperatorStringMatch,
											StringValue: "Test",
										},
									}
								},
								Assert: func(got *data.List[*data.SearchResult[*model.Product]], err error) {
									r.NoError(err)
									a.Len(got.List, 2)
								},
							},
							"empty-criteria-lists-all": {
								Arrange: func(ctx context.Context) (context.Context, []*data.SearchPredicate) {
									return ctx, nil
								},
								Assert: func(got *data.List[*data.SearchResult[*model.Product]], err error) {
									r.NoError(err)
									a.Len(got.List, 4)
								},
							},
						}
					default:
						panic(fmt.Errorf("unexpected product name: %s", md.Name))
					}
				},
			})
	})
}

func TestProductRepository_Crud(t *testing.T) {
	a := assert.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.ProductRepository) {
		datatesting.DoTestCrud[*model.Product, string](t, cp.GetContext(), unit,
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

func TestProductRepository_ListByCategory(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.ProductRepository) {

		ctx := cp.GetContext()

		got, err := unit.ListByCategory(ctx, "a", data.ListParams{})
		r.NoError(err)
		a.Len(got.List, 2)
	})
}

func TestProductRepository_GetSearchPredicates(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.ProductRepository) {

		ctx := cp.GetContext()

		got, err := unit.GetSearchPredicates(ctx)
		r.NoError(err)
		a.Equal([]*data.SearchPredicateDescriptor{
			{
				Name:    "@keywords",
				Label:   "Keywords",
				Virtual: true,
				Operators: []data.SearchOperator{
					data.SearchOperatorStringMatch,
				},
			},
			{
				Name:    "@query",
				Label:   "Query",
				Virtual: true,
				Operators: []data.SearchOperator{
					data.SearchOperatorStringMatch,
				},
			},
		}, got)
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

// TestProductRepository_AssociateTags covers the second edge on Product. The
// Tag edge declares its own ExecuteAdd and the Category edge does not, so this
// also asserts the generator put the hook on one edge and not the other: the
// tag add is idempotent, the category add is not.
func TestProductRepository_AssociateTags(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider,
		unit repository.ProductRepository,
		cr repository.CategoryRepository,
		tr repository.TagRepository,
	) {

		ctx := cp.GetContext()

		sku := uuid.New().String()
		tag := uuid.New().String()
		category := uuid.New().String()

		r.NoError(unit.Create(ctx, &model.Product{SKU: sku, Description: sku}))
		r.NoError(tr.Create(ctx, &model.Tag{Name: tag, Color: "red"}))
		r.NoError(cr.Create(ctx, &model.Category{Name: category, Description: category}))

		// Both edges write to their own table.

		r.NoError(unit.AssociateTags(ctx, sku, []string{tag}, nil))
		r.NoError(unit.AssociateCategories(ctx, sku, []string{category}, nil))

		got, err := unit.ListByTag(ctx, tag, data.ListParams{})
		r.NoError(err)
		a.Len(got.List, 1)

		got, err = unit.ListByCategory(ctx, category, data.ListParams{})
		r.NoError(err)
		a.Len(got.List, 1)

		// ProductTagExecuteAdd is ON CONFLICT DO NOTHING, so a repeat add on
		// the tag edge is a no-op rather than a primary key violation.

		r.NoError(unit.AssociateTags(ctx, sku, []string{tag}, nil))

		got, err = unit.ListByTag(ctx, tag, data.ListParams{})
		r.NoError(err)
		a.Len(got.List, 1)

		// The category edge took the generator default, which has no such
		// hook. If the generator had put ProductTagExecuteAdd on both edges
		// this would pass instead of failing.

		r.Error(unit.AssociateCategories(ctx, sku, []string{category}, nil))

		// Removal is unaffected, and reaches only its own edge.

		r.NoError(unit.AssociateTags(ctx, sku, nil, []string{tag}))

		got, err = unit.ListByTag(ctx, tag, data.ListParams{})
		r.NoError(err)
		a.Empty(got.List)

		got, err = unit.ListByCategory(ctx, category, data.ListParams{})
		r.NoError(err)
		a.Len(got.List, 1)

	})
}

func TestProductRepository_ListAllPagination(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.ProductRepository) {
		ctx := cp.GetContext()

		page1, err := unit.ListAll(ctx, data.ListParams{
			PageParams: &data.PageParams{Count: 2},
		})
		r.NoError(err)
		a.Len(page1.List, 2)
		a.NotEmpty(page1.NextPageToken, "expected NextPageToken on first page")

		page2, err := unit.ListAll(ctx, data.ListParams{
			PageParams: &data.PageParams{Count: 2, PageToken: page1.NextPageToken},
		})
		r.NoError(err)
		a.Len(page2.List, 2)

		// Rows on page2 must all sort strictly after page1's last row.
		lastOnPage1 := page1.List[len(page1.List)-1].SKU
		for _, p := range page2.List {
			a.Greater(p.SKU, lastOnPage1, "page2 row %q should sort after page1 last %q", p.SKU, lastOnPage1)
		}

		// Walk to the end with successive cursors; on the final page the
		// token must be empty.
		token := page2.NextPageToken
		for i := 0; token != "" && i < 100; i++ {
			next, err := unit.ListAll(ctx, data.ListParams{
				PageParams: &data.PageParams{Count: 2, PageToken: token},
			})
			r.NoError(err)
			token = next.NextPageToken
		}
		a.Empty(token, "pagination should terminate with an empty NextPageToken")
	})
}
