package repository_test

import (
	"testing"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/examples/data/repository"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestThemeRepository_Crud(t *testing.T) {
	a := assert.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.ThemeRepository) {
		datatesting.DoTestCrud[*model.Theme, string](t, model.WithTenant(cp.GetContext(), "2"), unit,
			&datatesting.CrudTestFixture[*model.Theme, string]{
				KeyExists:  "a",
				KeyMissing: "invalid",
				NewEntity: func() *model.Theme {
					return &model.Theme{}
				},
				ExtractKey: func(e *model.Theme) string {
					return e.Name
				},
				AssertDetailEntry: func(_ *testing.T, e *model.Theme) {
					a.NotEmpty(e.Name)
					a.NotEmpty(e.Description)
				},
				ModifyBeforeCreate: func(e *model.Theme) {
					e.Name = uuid.New().String()
					e.Description = "initial"
				},
				AssertAfterCreate: func(_ *testing.T, e *model.Theme) {
					a.Equal("initial", e.Description)
				},
				ModifyBeforeUpdate: func(e *model.Theme) {
					e.Description = "modified"
				},
				AssertAfterUpdate: func(_ *testing.T, e *model.Theme) {
					a.Equal("modified", e.Description)
				},
			})
	})
}
