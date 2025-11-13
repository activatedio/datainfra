package repository_test

import (
	"testing"

	"github.com/activatedio/datainfra/examples/data/repository"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCategoryRepository_Crud(t *testing.T) {
	r := require.New(t)
	a := assert.New(t)
	datatesting.Run(t, AppFixtures, func(cp datatesting.ContextProvider, unit repository.CategoryRepository) {
		ctx := cp.GetContext()
		got, err := unit.FindByKey(ctx, "key")
		r.NoError(err)
		a.NotNil(got)
	})
}
