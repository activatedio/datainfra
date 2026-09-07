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
	repogorm "github.com/activatedio/datainfra/examples/data/repository/gorm"
	"github.com/activatedio/datainfra/pkg/data"
	dgorm "github.com/activatedio/datainfra/pkg/data/gorm"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
)

// seedThemes creates n themes under a fresh tenant and returns a context
// scoped to that tenant. The theme repository is tenant-scoped, so rows
// seeded here are invisible to every other test sharing the database.
func seedThemes(t *testing.T, cp datatesting.ContextProvider, unit repository.ThemeRepository, n int) context.Context {
	ctx := model.WithTenant(cp.GetContext(), uuid.New().String())
	for i := 0; i < n; i++ {
		require.NoError(t, unit.Create(ctx, &model.Theme{
			Name:        fmt.Sprintf("theme-%04d", i),
			Description: "seeded",
		}))
	}
	return ctx
}

// An empty ListParams is a request for the first page, not for everything:
// the response must say so with a NextPageToken.
func TestThemeRepository_EmptyParamsIsFirstPageWithToken(t *testing.T) {
	datatesting.Run(t, AppFixtures, func(t *testing.T, cp datatesting.ContextProvider, unit repository.ThemeRepository) {
		a := assert.New(t)
		r := require.New(t)

		ctx := seedThemes(t, cp, unit, dgorm.DefaultPageSize+50)

		page1, err := unit.ListAll(ctx, data.ListParams{})
		r.NoError(err)
		a.Len(page1.List, dgorm.DefaultPageSize)
		r.NotEmpty(page1.NextPageToken, "a nil PageParams must still page: more rows exist, so a token is owed")

		page2, err := unit.ListAll(ctx, data.ListParams{PageParams: &data.PageParams{PageToken: page1.NextPageToken}})
		r.NoError(err)
		a.Len(page2.List, 50)
		a.Empty(page2.NextPageToken)

		a.Greater(page2.List[0].Name, page1.List[len(page1.List)-1].Name)

		// Under the page size there is no token at all.
		small := seedThemes(t, cp, unit, 3)
		got, err := unit.ListAll(small, data.ListParams{})
		r.NoError(err)
		a.Len(got.List, 3)
		a.Empty(got.NextPageToken)
	})
}

func TestThemeRepository_CollectAndExistsAny(t *testing.T) {
	datatesting.Run(t, AppFixtures, func(t *testing.T, cp datatesting.ContextProvider, unit repository.ThemeRepository) {
		a := assert.New(t)
		r := require.New(t)

		const n = dgorm.DefaultPageSize*2 + 30
		full := seedThemes(t, cp, unit, n)
		empty := model.WithTenant(cp.GetContext(), uuid.New().String())

		// Collect follows tokens to exhaustion and honours the ambient scope.
		all, err := data.Collect(full, unit.ListAll, data.CollectParams{})
		r.NoError(err)
		a.Len(all, n)
		for i, th := range all {
			a.Equal(fmt.Sprintf("theme-%04d", i), th.Name)
		}

		// A smaller page size changes the number of round trips, not the answer.
		all, err = data.Collect(full, unit.ListAll, data.CollectParams{PageSize: 7})
		r.NoError(err)
		a.Len(all, n)

		// Past the ceiling the caller gets an error and nothing else.
		got, err := data.Collect(full, unit.ListAll, data.CollectParams{Max: n - 1})
		var limit data.CollectLimitExceeded
		r.ErrorAs(err, &limit)
		a.Nil(got)
		a.Equal(n-1, limit.Max)

		// Exactly the ceiling is fine.
		all, err = data.Collect(full, unit.ListAll, data.CollectParams{Max: n})
		r.NoError(err)
		a.Len(all, n)

		// ExistsAny with no criteria of its own is answered by the ambient
		// tenant scope alone, and agrees with Collect's emptiness.
		ok, err := data.ExistsAny(full, unit.ListAll, nil)
		r.NoError(err)
		a.True(ok)

		ok, err = data.ExistsAny(empty, unit.ListAll, nil)
		r.NoError(err)
		a.False(ok)

		none, err := data.Collect(empty, unit.ListAll, data.CollectParams{})
		r.NoError(err)
		a.Empty(none)
	})
}

// A template without key columns cannot hand out tokens, so when more rows
// match than fit in the page it must fail loudly rather than return a
// prefix as if it were the whole result.
func TestKeylessTemplate_OverflowIsAnError(t *testing.T) {
	datatesting.Run(t, AppFixtures, func(t *testing.T, cp datatesting.ContextProvider, unit repository.ThemeRepository) {
		a := assert.New(t)
		r := require.New(t)

		keyless := dgorm.NewMappingTemplate[*model.Theme, *repogorm.ThemeInternal](dgorm.MappingTemplateParams[*model.Theme, *repogorm.ThemeInternal]{
			ContextScope: repogorm.WithTenantScope(),
			Table:        "themes2",
			ToInternal:   func(m *model.Theme) *repogorm.ThemeInternal { return &repogorm.ThemeInternal{Theme: m} },
			FromInternal: func(m *repogorm.ThemeInternal) *model.Theme { return m.Theme },
		})

		// Within the page: the full set comes back and there is no token.
		small := seedThemes(t, cp, unit, 5)
		got, err := keyless.DoList(small, nil, data.ListParams{})
		r.NoError(err)
		a.Len(got.List, 5)
		a.Empty(got.NextPageToken)

		// Past the page: an error naming the table and the limit.
		big := seedThemes(t, cp, unit, dgorm.DefaultPageSize+1)
		got, err = keyless.DoList(big, nil, data.ListParams{})
		var overflow dgorm.UnpagedOverflowError
		r.ErrorAs(err, &overflow)
		a.Nil(got)
		a.Equal("themes2", overflow.Table)
		a.Equal(dgorm.DefaultPageSize, overflow.Limit)
		a.Contains(err.Error(), "KeyColumns")

		// An explicit smaller Count moves the threshold, and overflows the same way.
		got, err = keyless.DoList(small, nil, data.ListParams{PageParams: &data.PageParams{Count: 3}})
		r.ErrorAs(err, &overflow)
		a.Nil(got)
		a.Equal(3, overflow.Limit)

		// A page token is meaningless to a keyless template.
		_, err = keyless.DoList(small, nil, data.ListParams{PageParams: &data.PageParams{PageToken: "anything"}})
		r.Error(err)
		a.Contains(err.Error(), "invalid page token")
	})
}
