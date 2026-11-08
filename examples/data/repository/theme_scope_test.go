package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/examples/data/repository"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
)

// The theme key is a name that repeats across tenants: themes2's primary key
// is (tenant_id, name) and the model's gorm primary key is Name alone. A write
// under one tenant must never reach the same-named row of another.
func TestThemeRepository_WritesStayInScope(t *testing.T) {

	type arrangement struct {
		ctx, other context.Context
		name       string
	}

	cases := map[string]struct {
		act    func(ctx context.Context, unit repository.ThemeRepository, name string) error
		gone   bool
		assert func(a *assert.Assertions, mine, theirs *model.Theme)
	}{
		"delete by key": {
			act: func(ctx context.Context, unit repository.ThemeRepository, name string) error {
				return unit.Delete(ctx, name)
			},
			gone: true,
		},
		"delete entity": {
			act: func(ctx context.Context, unit repository.ThemeRepository, name string) error {
				return unit.DeleteEntity(ctx, &model.Theme{Name: name})
			},
			gone: true,
		},
		"update": {
			act: func(ctx context.Context, unit repository.ThemeRepository, name string) error {
				return unit.Update(ctx, &model.Theme{Name: name, Description: "modified"})
			},
			assert: func(a *assert.Assertions, mine, _ *model.Theme) {
				a.Equal("modified", mine.Description)
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			datatesting.Run(t, AppFixtures, func(t *testing.T, cp datatesting.ContextProvider, unit repository.ThemeRepository) {
				a := assert.New(t)
				r := require.New(t)

				arrange := func() arrangement {
					ar := arrangement{
						ctx:   model.WithTenant(cp.GetContext(), "2"),
						other: model.WithTenant(cp.GetContext(), "1"),
						name:  uuid.New().String(),
					}
					r.NoError(unit.Create(ar.ctx, &model.Theme{Name: ar.name, Description: "mine"}))
					r.NoError(unit.Create(ar.other, &model.Theme{Name: ar.name, Description: "theirs"}))
					return ar
				}
				ar := arrange()

				r.NoError(v.act(ar.ctx, unit, ar.name))

				mine, err := unit.FindByKey(ar.ctx, ar.name)
				r.NoError(err)
				theirs, err := unit.FindByKey(ar.other, ar.name)
				r.NoError(err)

				r.NotNil(theirs, "a write under tenant 2 removed tenant 1's row")
				a.Equal("theirs", theirs.Description, "a write under tenant 2 changed tenant 1's row")
				if v.gone {
					a.Nil(mine)
				}
				if v.assert != nil {
					r.NotNil(mine)
					v.assert(a, mine, theirs)
				}
			})
		})
	}
}
