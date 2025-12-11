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

func TestCategoryRepository_Crud(t *testing.T) {
	a := assert.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.CategoryRepository) {
		datatesting.DoTestCrud[*model.Category, string](t, cp.GetContext(), unit,
			&datatesting.CrudTestFixture[*model.Category, string]{
				KeyExists:  "a",
				KeyMissing: "invalid",
				NewEntity: func() *model.Category {
					return &model.Category{}
				},
				ExtractKey: func(e *model.Category) string {
					return e.Name
				},
				AssertDetailEntry: func(_ *testing.T, e *model.Category) {
					a.NotEmpty(e.Name)
					a.NotEmpty(e.Description)
				},
				ModifyBeforeCreate: func(e *model.Category) {
					e.Name = uuid.New().String()
					e.Description = "initial"
				},
				AssertAfterCreate: func(_ *testing.T, e *model.Category) {
					a.Equal("initial", e.Description)
				},
				ModifyBeforeUpdate: func(e *model.Category) {
					e.Description = "modified"
				},
				AssertAfterUpdate: func(_ *testing.T, e *model.Category) {
					a.Equal("modified", e.Description)
				},
			})
	})
}

func TestCategoryRepository_FilterKeys(t *testing.T) {
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.CategoryRepository) {
		datatesting.DoTestFilterKeys[string, repository.CategoryRepository](t, cp.GetContext(), unit,
			&datatesting.FilterKeysTestFixture[string]{
				KeyExists:  "a",
				KeyMissing: "invlaid",
			})
	})
}

func TestCategoryRepository_ListByProduct(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.CategoryRepository) {

		ctx := cp.GetContext()

		got, err := unit.ListByProduct(ctx, "1", data.ListParams{})
		r.NoError(err)
		a.Len(got.List, 1)

	})
}
