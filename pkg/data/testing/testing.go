package testing

import (
	"context"
	"reflect"
	"testing"

	"github.com/activatedio/datainfra/pkg/data"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
)

// RandomLabels generates and returns a set of random label key-value pairs with unique UUID values for each key.
func RandomLabels() data.Labels {
	return map[string]string{
		// TODO - better to have another uuid provider
		"a1": uuid.New().String(),
		"a2": uuid.New().String(),
	}
}

// Run executes the given test cases using the list of AppFixture, invoking the provided functions and values.
func Run(t *testing.T, fixtures []AppFixture, toInvoke any, toProvide ...any) {

	for _, fix := range fixtures {

		res := fix.GetApp(t, toInvoke, toProvide...)

		t.Run(res.Name, func(_ *testing.T) {
			res.App.RequireStart()

			res.App.RequireStop()
		})

	}
}

// ListAssertion defines the expected conditions for validating lists of type E during tests.
type ListAssertion[E any] struct {
	// ExpectedCount specifies the number of elements expected in the list.
	ExpectedCount int
	// AssertListEntry defines a function to assert individual entries in the list using *testing.T.
	AssertListEntry func(t *testing.T, e E)
}

// SelectAssertion represents a validation on a select query with an expression and its expected result count.
type SelectAssertion struct {
	Expression    string
	ExpectedCount int
}

// CrudTestFixture represents a generic fixture for testing CRUD operations on entities of type E with keys of type K.
type CrudTestFixture[E any, K comparable] struct {
	NewEntity          func() E
	KeyExists          K
	KeyMissing         K
	ExtractKey         func(e E) K
	SelectAssertions   []SelectAssertion
	ListAssertion      *ListAssertion[E]
	AssertDetailEntry  func(t *testing.T, e E)
	ModifyBeforeCreate func(e E)
	AssertAfterCreate  func(t *testing.T, e E)
	ModifyBeforeUpdate func(e E)
	AssertAfterUpdate  func(t *testing.T, e E)
}

// DoTestCrud performs a comprehensive CRUD test for a generic repository using provided test fixtures.
func DoTestCrud[E any, K comparable](t *testing.T,
	ctx context.Context, unit data.CrudTemplate[E, K], fixture *CrudTestFixture[E, K]) { //nolint:revive // okay to have ctx second for a test

	for _, sa := range fixture.SelectAssertions {

		l, err := labels.Parse(sa.Expression)

		if err != nil {
			panic(err)
		}

		list, err := unit.ListAll(ctx, data.ListParams{
			Selector: l,
		})

		require.NoError(t, err)
		assert.Len(t, list.List, sa.ExpectedCount, sa.Expression)
	}

	if fixture.ListAssertion != nil {

		la := fixture.ListAssertion

		list, err := unit.ListAll(ctx, data.ListParams{})

		require.NoError(t, err)
		assert.Len(t, list.List, la.ExpectedCount)

		assert.NotNil(t, list.List)
		for _, v := range list.List {
			la.AssertListEntry(t, v)
		}
	}

	got, err := unit.FindByKey(ctx, fixture.KeyMissing)

	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = unit.FindByKey(ctx, fixture.KeyExists)

	require.NoError(t, err)
	assert.NotNil(t, got)

	fixture.AssertDetailEntry(t, got)

	// Create with bad labels
	got = fixture.NewEntity()
	if HasLabels(got) {
		fixture.ModifyBeforeCreate(got)
		SetBadLabels(got)
		err = unit.Create(ctx, got)
		assert.Contains(t, err.Error(), "name part must consist")
	}
	// Create
	got = fixture.NewEntity()
	fixture.ModifyBeforeCreate(got)
	err = unit.Create(ctx, got)
	require.NoError(t, err)

	fixture.AssertAfterCreate(t, got)

	err = unit.Create(ctx, got)
	assert.True(t, errors.Is(err, data.EntityAlreadyExists{}))

	key := fixture.ExtractKey(got)

	got2, err := unit.FindByKey(ctx, key)
	require.NoError(t, err)

	fixture.AssertAfterCreate(t, got2)

	if fixture.AssertAfterUpdate != nil && fixture.ModifyBeforeUpdate != nil {

		fixture.ModifyBeforeUpdate(got)

		err = unit.Update(ctx, got)

		require.NoError(t, err)

		fixture.AssertAfterUpdate(t, got)

		if HasLabels(got) {
			SetBadLabels(got)
			err = unit.Update(ctx, got)
			assert.Contains(t, err.Error(), "name part must consist")
		}

		got2, err = unit.FindByKey(ctx, fixture.ExtractKey(got))
		require.NoError(t, err)
		fixture.AssertAfterUpdate(t, got2)

	}

	err = unit.Delete(ctx, key)
	require.NoError(t, err)

	got3, err := unit.FindByKey(ctx, key)

	require.NoError(t, err)
	assert.Nil(t, got3)

}

// SetBadLabels modifies the "Labels" field of the provided struct to set intentionally malformed key-value pairs.
func SetBadLabels(got any) {

	f := reflect.ValueOf(got).Elem().FieldByName("Labels")
	f.Set(reflect.ValueOf(map[string]string{
		" b a d k e y": "__--**&&bdValue",
	}))
}

// HasLabels checks if the provided value has a struct field named "Labels". Returns true if the field exists, otherwise false.
func HasLabels(got any) bool {
	_, ok := reflect.TypeOf(got).Elem().FieldByName("Labels")
	return ok
}

// FilterKeysTestFixture is a testing fixture for validating FilterKeysTemplate implementations with generic key support.
type FilterKeysTestFixture[K comparable] struct {
	// ArrangeContext allows preparation or alteration of the execution context for tests.
	ArrangeContext func(context.Context) context.Context
	// KeyExists is a key expected to be recognized as existing within the context of FilterKeys.
	KeyExists K
	// KeyMissing is a key expected to be unrecognized or missing within the context of FilterKeys.
	KeyMissing K
}

// DoTestFilterKeys performs a comprehensive test for a FilterKeysTemplate implementation using the provided fixture.
func DoTestFilterKeys[K comparable, T data.FilterKeysTemplate[K]](t *testing.T,
	ctx context.Context, unit data.FilterKeysTemplate[K], fixture *FilterKeysTestFixture[K]) { //nolint:revive // okay to have ctx second for a test

	if f := fixture.ArrangeContext; f != nil {
		ctx = fixture.ArrangeContext(ctx)
	}

	got, err := unit.FilterKeys(ctx, []K{fixture.KeyExists, fixture.KeyMissing})

	require.NoError(t, err)
	assert.Equal(t, []K{fixture.KeyExists}, got)
}

// SearchTestFixtureEntry represents a single test case for a SearchTemplate implementation.
type SearchTestFixtureEntry[E any] struct {
	Arrange func(ctx context.Context) (context.Context, []*data.SearchPredicate)
	Assert  func(got *data.List[*data.SearchResult[E]], err error)
}

// SearchTestFixture represents a generic testing fixture for validating SearchTemplate implementations.
type SearchTestFixture[E any, T data.SearchTemplate[E]] struct {
	ArrangeContext func(context.Context) context.Context
	Init           func() func(ctx context.Context, unit T) error
	Teardown       func() func(ctx context.Context, unit T) error
	FixtureEntries func() map[string]*SearchTestFixtureEntry[E]
}

// DoTestSearch performs a comprehensive test for a SearchTemplate implementation using the provided fixture.
func DoTestSearch[E any, T data.SearchTemplate[E]](t *testing.T, ctx context.Context, unit T, fixture *SearchTestFixture[E, T]) { //nolint:revive // okay to have ctx second for a test
	if f := fixture.Init; f != nil {
		require.NoError(t, f()(ctx, unit))
	}

	if f := fixture.ArrangeContext; f != nil {
		ctx = fixture.ArrangeContext(ctx)
	}

	for k2, v2 := range fixture.FixtureEntries() {
		t.Run(k2, func(_ *testing.T) {

			_ctx, preds := v2.Arrange(ctx)

			got, err := unit.Search(_ctx, preds, nil)
			v2.Assert(got, err)
		})
	}

	if f := fixture.Teardown; f != nil {
		require.NoError(t, f()(ctx, unit))
	}
}
