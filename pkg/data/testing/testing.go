package testing

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/activatedio/datainfra/pkg/data"
)

// RandomLabels generates and returns a set of random label key-value pairs with unique UUID values for each key.
func RandomLabels() data.Labels {
	return map[string]string{
		// TODO - better to have another uuid provider
		"a1": uuid.New().String(),
		"a2": uuid.New().String(),
	}
}

type invoker struct {
	f    reflect.Value
	args []reflect.Type
	hasT bool // true when the first param of f is *testing.T
}

func (i invoker) getArgs() []any {
	res := make([]any, len(i.args))
	for j, a := range i.args {
		res[j] = reflect.New(a).Interface()
	}
	return res
}

func (i invoker) invoke(args []any, t *testing.T) {
	argValues := make([]reflect.Value, 0, len(args)+1)
	if i.hasT {
		argValues = append(argValues, reflect.ValueOf(t))
	}
	for _, a := range args {
		argValues = append(argValues, reflect.ValueOf(a).Elem())
	}
	i.f.Call(argValues)
}

func newInvoker(invokeFunc any) invoker {
	iv := reflect.ValueOf(invokeFunc)

	if iv.Kind() != reflect.Func {
		panic("toInvoke must be a function")
	}

	testingTType := reflect.TypeOf((*testing.T)(nil))
	hasT := iv.Type().NumIn() > 0 && iv.Type().In(0) == testingTType

	start := 0
	if hasT {
		start = 1
	}

	var args []reflect.Type
	for i := start; i < iv.Type().NumIn(); i++ {
		args = append(args, iv.Type().In(i))
	}

	return invoker{iv, args, hasT}
}

// RunOption configures behavior of Run.
type RunOption func(*runConfig)

type runConfig struct {
	provide  []any
	rootOpts []fx.Option
	logger   fx.Option
}

// WithProvide supplies values to the test fx app's root scope.
func WithProvide(provide ...any) RunOption {
	return func(c *runConfig) {
		c.provide = append(c.provide, provide...)
	}
}

// WithRootOptions appends fx.Options at the root scope of the test app.
// Use this for options other than the fx event logger — for the logger,
// use WithLogger so it is applied after fxtest's auto-injected TB logger
// and therefore takes effect.
func WithRootOptions(opts ...fx.Option) RunOption {
	return func(c *runConfig) {
		c.rootOpts = append(c.rootOpts, opts...)
	}
}

// WithLogger replaces the default fx event logger (fx.NopLogger) used by Run.
// Pass an fx.Option that installs a logger, e.g. fx.WithLogger(...) or
// fxtest.WithTestLogger(t), to route fx events somewhere visible during
// debugging.
func WithLogger(logger fx.Option) RunOption {
	return func(c *runConfig) {
		c.logger = logger
	}
}

// Run executes the given test cases using the list of AppFixture, invoking the provided functions and values.
// Each fixture runs as a parallel sub-test. If toInvoke's first parameter is *testing.T, the sub-test's t
// is injected as that argument so assertions and sub-sub-tests are scoped to the right test node.
//
// By default fx event output is silenced via fx.NopLogger. Use WithLogger to override.
func Run(t *testing.T, fixtures []AppFixture, toInvoke any, opts ...RunOption) {

	cfg := &runConfig{
		logger: fx.NopLogger,
	}
	for _, o := range opts {
		o(cfg)
	}

	for _, fix := range fixtures {

		// Each backend runs as its own parallel subtest, and the fixture is
		// asked with that subtest's t: the store hold, the reverse of this
		// test's layers and the drop of a dedicated store all belong to the
		// subtest and run when it ends. Asking with the parent t would make
		// every backend's subtest — and every Run a test function makes —
		// look like one holder.
		name := fix.Name()

		t.Run(name, func(subt *testing.T) {
			subt.Parallel()

			res := fix.GetApp(subt, cfg.provide...)

			inv := newInvoker(toInvoke)
			invokeArgs := inv.getArgs()
			fxOpts := []fx.Option{
				res.App,
				fx.Populate(invokeArgs...),
			}
			fxOpts = append(fxOpts, cfg.rootOpts...)
			if cfg.logger != nil {
				fxOpts = append(fxOpts, cfg.logger)
			}

			app := fxtest.New(subt, fxOpts...)

			app.RequireStart()

			inv.invoke(invokeArgs, subt)

			app.RequireStop()
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

	exists, err := unit.ExistsByKey(ctx, fixture.KeyMissing)
	require.NoError(t, err)
	assert.False(t, exists, "ExistsByKey(KeyMissing) must be false")

	got, err = unit.FindByKey(ctx, fixture.KeyExists)

	require.NoError(t, err)
	assert.NotNil(t, got)

	exists, err = unit.ExistsByKey(ctx, fixture.KeyExists)
	require.NoError(t, err)
	assert.True(t, exists, "ExistsByKey(KeyExists) must be true")

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
