package fs_test

import (
	"io"
	"io/fs"
	"testing"
	"text/template"

	corefs "github.com/activatedio/datainfra/pkg/data/fs"
	"github.com/activatedio/datainfra/pkg/data/fs/testdata/simple1"
	"github.com/activatedio/datainfra/pkg/data/fs/testdata/subdirs1"
	"github.com/activatedio/datainfra/pkg/symbols"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Dummy struct {
	A string
	B string
}

func TestTemplateFS(t *testing.T) {

	type s struct {
		arrange func() []corefs.TemplateOption
		assert  func(got fs.FS, err error)
	}

	data := &Dummy{
		A: "1",
		B: "2",
	}

	funcs := func(syms symbols.Symbols) template.FuncMap {
		return map[string]interface{}{
			"symbolString": func(name string) string {
				return syms.MustGetString(name)
			},
		}
	}

	cases := map[string]s{
		"simple1": {
			arrange: func() []corefs.TemplateOption {
				return []corefs.TemplateOption{corefs.WithSource(simple1.Files),
					corefs.WithSymbolSource(simple1.SymbolSources()),
					corefs.WithData(data),
					corefs.WithFuncs(funcs),
				}
			},
			assert: func(got fs.FS, err error) {
				require.NoError(t, err)
				a, err := got.Open("a.txt")
				require.NoError(t, err)
				defer a.Close()
				b, err := got.Open("b.txt")
				require.NoError(t, err)
				defer b.Close()

				ab, err := io.ReadAll(a)
				require.NoError(t, err)
				assert.Equal(t, "Value1: 1 2 value1\n", string(ab))
				bb, err := io.ReadAll(b)
				assert.Equal(t, "Value2: value2\n", string(bb))
				require.NoError(t, err)
			},
		},
		"subdirs1": {
			arrange: func() []corefs.TemplateOption {
				return []corefs.TemplateOption{
					corefs.WithSource(subdirs1.Files),
					corefs.WithSymbolSource(subdirs1.SymbolSources()),
					corefs.WithFuncs(funcs),
				}
			},
			assert: func(got fs.FS, err error) {
				require.NoError(t, err)
				a, err := got.Open("a/aa.txt")
				require.NoError(t, err)
				defer a.Close()
				b, err := got.Open("b/bb.txt")
				require.NoError(t, err)
				defer b.Close()

				ab, err := io.ReadAll(a)
				require.NoError(t, err)
				assert.Equal(t, "A Value1: value1\n", string(ab))
				bb, err := io.ReadAll(b)
				assert.Equal(t, "B Value2: value2\n", string(bb))
				require.NoError(t, err)
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(_ *testing.T) {

			opts := v.arrange()
			v.assert(corefs.TemplateFS(opts...))
		})
	}
}
