package repository_test

import (
	"testing"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/examples/data/repository"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestLocationRepository_Crud(t *testing.T) {
	a := assert.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.LocationRepository) {
		datatesting.DoTestCrud[*model.Location, model.LocationKey](t, cp.GetContext(), unit,
			&datatesting.CrudTestFixture[*model.Location, model.LocationKey]{
				KeyExists: model.LocationKey{City: "Seattle", State: "WA"},
				KeyMissing: model.LocationKey{
					City:  "invalid",
					State: "invalid",
				},
				NewEntity: func() *model.Location {
					return &model.Location{}
				},
				ExtractKey: func(e *model.Location) model.LocationKey {
					return model.LocationKey{
						City:  e.City,
						State: e.State,
					}
				},

				AssertDetailEntry: func(_ *testing.T, e *model.Location) {
					a.NotEmpty(e.Latitude)
					a.NotEmpty(e.Longitude)
				},
				ModifyBeforeCreate: func(e *model.Location) {
					e.City = uuid.New().String()
					e.State = uuid.New().String()
					e.Latitude = 1
					e.Longitude = 1
				},
				AssertAfterCreate: func(_ *testing.T, _ *model.Location) {
				},
				ModifyBeforeUpdate: func(e *model.Location) {
					e.Longitude = 2
				},
				AssertAfterUpdate: func(_ *testing.T, _ *model.Location) {
				},
			})
	})
}
