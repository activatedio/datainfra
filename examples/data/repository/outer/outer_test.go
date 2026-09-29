package outer_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/examples/data/repository"
	"github.com/activatedio/datainfra/examples/data/repository/outer"
	pkgouter "github.com/activatedio/datainfra/pkg/data/outer"
)

// memProducts is an inner ProductRepository over a map: enough of one for
// the chain's writes. Methods it does not override panic on the nil
// embedded interface, so a proxy calling something unexpected fails loudly.
type memProducts struct {
	repository.ProductRepository
	rows  map[string]model.Product
	calls []string
}

func (m *memProducts) FindByKey(_ context.Context, k string) (*model.Product, error) {
	if p, ok := m.rows[k]; ok {
		return &p, nil
	}
	return nil, nil
}

func (m *memProducts) Create(_ context.Context, p *model.Product) error {
	m.calls = append(m.calls, "Create")
	m.rows[p.SKU] = *p
	return nil
}

func (m *memProducts) Update(_ context.Context, p *model.Product) error {
	m.calls = append(m.calls, "Update")
	m.rows[p.SKU] = *p
	return nil
}

func (m *memProducts) Delete(_ context.Context, k string) error {
	m.calls = append(m.calls, "Delete")
	delete(m.rows, k)
	return nil
}

const update = "update"

type sunk struct {
	m  pkgouter.MethodMetadata
	cl pkgouter.Changelog
}

type recorder struct{ got []sunk }

func (r *recorder) Sink(_ context.Context, m pkgouter.MethodMetadata, cl pkgouter.Changelog) error {
	r.got = append(r.got, sunk{m, cl})
	return nil
}

func chain(t *testing.T, sink pkgouter.DiffSink) (repository.ProductRepository, *memProducts) {
	t.Helper()
	inner := &memProducts{rows: map[string]model.Product{}}
	var repo repository.ProductRepository
	opts := []fx.Option{
		fx.Provide(pkgouter.TagInner(func() repository.ProductRepository { return inner })),
		fx.Provide(outer.NewProductRepository),
		fx.Populate(&repo),
	}
	if sink != nil {
		opts = append(opts, fx.Supply(fx.Annotate(sink, fx.As(new(pkgouter.DiffSink)))))
	}
	app := fxtest.New(t, opts...)
	app.RequireStart()
	t.Cleanup(app.RequireStop)
	return repo, inner
}

func TestAuditedChain(t *testing.T) {
	ctx := context.Background()
	rec := &recorder{}
	repo, inner := chain(t, rec)

	require.NoError(t, repo.Create(ctx, &model.Product{SKU: "a", Description: "first"}))
	require.NoError(t, repo.Update(ctx, &model.Product{SKU: "a", Description: "second"}))
	require.NoError(t, repo.Delete(ctx, "a"))
	// A delete of a row that is not there changes nothing and records nothing.
	require.NoError(t, repo.Delete(ctx, "missing"))

	require.Len(t, rec.got, 3)

	assert.Equal(t, pkgouter.MethodMetadata{Source: "product", MethodName: "Create", TargetID: "a"}, rec.got[0].m)
	assert.Contains(t, rec.got[0].cl, pkgouter.Change{Type: "create", Path: []string{"Description"}, To: "first"})

	assert.Equal(t, "Update", rec.got[1].m.MethodName)
	assert.Equal(t, pkgouter.Changelog{{Type: update, Path: []string{"Description"}, From: "first", To: "second"}}, rec.got[1].cl)

	assert.Equal(t, pkgouter.MethodMetadata{Source: "product", MethodName: "Delete", TargetID: "a"}, rec.got[2].m)
	assert.Contains(t, rec.got[2].cl, pkgouter.Change{Type: "delete", Path: []string{"Description"}, From: "second"})

	assert.Equal(t, []string{"Create", "Update", "Delete", "Delete"}, inner.calls)
}

func TestValidatorRefusesBeforeTheWrite(t *testing.T) {
	rec := &recorder{}
	repo, inner := chain(t, rec)

	require.EqualError(t, repo.Create(context.Background(), &model.Product{}), "product: sku is required")
	assert.Empty(t, inner.calls, "a refused write never reaches the inner repository")
	assert.Empty(t, rec.got, "a refused write records nothing")
}

func TestDefaultSinkWhenTheGraphHasNone(t *testing.T) {
	repo, inner := chain(t, nil)

	require.NoError(t, repo.Create(context.Background(), &model.Product{SKU: "a"}))
	assert.Equal(t, []string{"Create"}, inner.calls)
}

func TestRedacted(t *testing.T) {
	cl := pkgouter.Changelog{
		{Type: update, Path: []string{"Secret"}, From: "old", To: "new"},
		{Type: update, Path: []string{"Name"}, From: "a", To: "b"},
	}
	assert.Equal(t, pkgouter.Changelog{
		{Type: update, Path: []string{"Secret"}, From: "[redacted]", To: "[redacted]"},
		{Type: update, Path: []string{"Name"}, From: "a", To: "b"},
	}, cl.Redacted([]string{"Secret"}))
}
