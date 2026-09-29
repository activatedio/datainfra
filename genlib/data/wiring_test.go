package data_test

import (
	"fmt"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/assert"

	"github.com/activatedio/datainfra/genlib/data"
)

func TestFXWiringWithRepositoryName(t *testing.T) {
	render := func(w data.Wiring) string {
		f := jen.NewFile("x")
		refs := []jen.Code{jen.Id("NewDB")}
		if rw, ok := w.(data.RepositoryWiring); ok {
			refs = append(refs, rw.WrapRepository(jen.Id("NewThingRepository")))
		}
		w.EmitIndex(f, refs)
		return fmt.Sprintf("%#v", f)
	}

	plain := render(data.FXWiring("m"))
	assert.Contains(t, plain, "fx.Provide(NewDB, NewThingRepository)")

	named := render(data.FXWiring("m", data.WithRepositoryName("inner")))
	assert.Contains(t, named, "fx.Provide(NewDB, fx.Annotate(NewThingRepository, fx.ResultTags(\"name:\\\"inner\\\"\")))")
}
