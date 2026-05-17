package repository_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/examples/data/repository"
	"github.com/activatedio/datainfra/pkg/data"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
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
					return e.Key
				},

				AssertDetailEntry: func(_ *testing.T, e *model.Location) {
					a.NotEmpty(e.Latitude)
					a.NotEmpty(e.Longitude)
				},
				ModifyBeforeCreate: func(e *model.Location) {
					e.Key = model.LocationKey{City: uuid.New().String(), State: uuid.New().String()}
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

func TestLocationRepository_ListAllPaginationComposite(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.LocationRepository) {
		ctx := cp.GetContext()

		// Seed deterministic rows whose (city, state) sort gives a known
		// lexicographic order: (Austin, TX) < (Boston, MA) < (Chicago, IL) <
		// (Denver, CO). Combined with the existing seed (San Francisco/CA,
		// Seattle/WA) we have >= 4 unique composite keys to paginate over.
		seed := []*model.Location{
			{Key: model.LocationKey{City: "Austin", State: "TX"}, Latitude: 1, Longitude: 1},
			{Key: model.LocationKey{City: "Boston", State: "MA"}, Latitude: 1, Longitude: 1},
			{Key: model.LocationKey{City: "Chicago", State: "IL"}, Latitude: 1, Longitude: 1},
			{Key: model.LocationKey{City: "Denver", State: "CO"}, Latitude: 1, Longitude: 1},
		}
		for _, l := range seed {
			r.NoError(unit.Create(ctx, l))
		}

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

		// Page 2 rows must all sort strictly after page 1's last row in
		// (city, state) lexicographic order.
		last := page1.List[len(page1.List)-1].Key
		for _, l := range page2.List {
			cmp := l.Key.City + "\x00" + l.Key.State
			cmpLast := last.City + "\x00" + last.State
			a.Greater(cmp, cmpLast, "page2 row %v should sort after page1 last %v", l.Key, last)
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
		a.Empty(token, "composite-key pagination should terminate with an empty NextPageToken")
	})
}
