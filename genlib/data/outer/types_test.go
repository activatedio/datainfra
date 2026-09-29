package outer

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/assert"

	"github.com/activatedio/datainfra/pkg/data"
)

type sample struct{ Name string }

func TestTypeCode(t *testing.T) {
	cases := map[string]reflect.Type{
		"string":                    reflect.TypeFor[string](),
		"error":                     reflect.TypeFor[error](),
		"any":                       reflect.TypeFor[any](),
		"context.Context":           reflect.TypeFor[context.Context](),
		"[]*outer.sample":           reflect.TypeFor[[]*sample](),
		"map[string][]int":          reflect.TypeFor[map[string][]int](),
		"*data.List[*outer.sample]": reflect.TypeFor[*data.List[*sample]](),
		"*data.List[*data.SearchResult[*outer.sample]]": reflect.TypeFor[*data.List[*data.SearchResult[*sample]]](),
	}
	for want, typ := range cases {
		f := jen.NewFilePath("example.com/other")
		f.Var().Id("v").Add(typeCode(typ))
		assert.Contains(t, fmt.Sprintf("%#v", f), "var v "+want, want)
	}
}

func TestSplitArgs(t *testing.T) {
	assert.Equal(t, []string{"a.B[c.D,e.F]", "*g.H"}, splitArgs("a.B[c.D,e.F],*g.H"))
}
