package data_test

import (
	"reflect"
	"testing"

	"github.com/activatedio/datainfra/genlib/data"
	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/assert"
)

type Dummy struct {
	Key string `data:"key"`
}

type Wrapper struct {
	Dummy
}

func TestEntry_GetJenHelper(t *testing.T) {

	cases := []struct {
		name   string
		input  data.Entry
		verify func(data.JenHelper)
	}{
		{
			name: "simple",
			input: data.Entry{
				Type: reflect.TypeFor[Dummy](),
			},
			verify: func(got data.JenHelper) {

				assert.Equal(t, "Dummy", got.StructName)
				assert.Equal(t, "DummyRepository", got.InterfaceName)
				assert.Equal(t, jen.Qual(reflect.TypeFor[Dummy]().PkgPath(), reflect.TypeFor[Dummy]().Name()), got.StructType)
				assert.Len(t, got.KeyFields, 1)
			},
		},
		{
			name: "wrapped",
			input: data.Entry{
				Type: reflect.TypeFor[Wrapper](),
			},
			verify: func(got data.JenHelper) {

				assert.Equal(t, "Wrapper", got.StructName)
				assert.Equal(t, "WrapperRepository", got.InterfaceName)
				assert.Equal(t, jen.Qual(reflect.TypeFor[Wrapper]().PkgPath(), reflect.TypeFor[Wrapper]().Name()), got.StructType)
				assert.Len(t, got.KeyFields, 1)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(_ *testing.T) {
			tt.verify(tt.input.GetJenHelper())
		})
	}
}
