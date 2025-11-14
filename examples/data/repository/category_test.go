package repository_test

import (
	"testing"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/examples/data/repository"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCategoryRepository_Crud(t *testing.T) {
	a := assert.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.CategoryRepository) {
		datatesting.DoTestCrudRepository[*model.Category, string](t, cp.GetContext(), unit,
			&datatesting.CrudTestFixture[*model.Category, string]{
				KeyExists:  "key",
				KeyMissing: "invalid",
				NewEntity: func() *model.Category {
					return &model.Category{}
				},
				ExtractKey: func(e *model.Category) string {
					return e.Name
				},
				AssertDetailEntry: func(t *testing.T, e *model.Category) {
					a.NotEmpty(e.Name)
					a.NotEmpty(e.Description)
				},
				ModifyBeforeCreate: func(e *model.Category) {
					e.Name = uuid.New().String()
					e.Description = "initial"
				},
				AssertAfterCreate: func(t *testing.T, e *model.Category) {
					a.Equal("initial", e.Description)
				},
				ModifyBeforeUpdate: func(e *model.Category) {
					e.Description = "modified"
				},
				AssertAfterUpdate: func(t *testing.T, e *model.Category) {
					a.Equal("modified", e.Description)
				},
			})
	})
}
