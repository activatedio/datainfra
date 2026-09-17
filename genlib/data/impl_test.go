package data_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/activatedio/datainfra/genlib/data"
)

// marker stands in for an implementation type a parent can carry more than
// one of — Associate is the real one.
type marker struct {
	Name string
}

type otherMarker struct{}

func entryWithMarkers(names ...string) *data.Entry {
	impls := []any{otherMarker{}}
	for _, n := range names {
		impls = append(impls, marker{Name: n})
	}
	return &data.Entry{
		Type:            reflect.TypeFor[Dummy](),
		Implementations: impls,
	}
}

func TestGetImplementations(t *testing.T) {

	cases := []struct {
		name  string
		entry *data.Entry
		opts  []data.ImplementationOption[marker]
		want  []marker
	}{
		{
			name:  "no predicate returns every implementation of the type",
			entry: entryWithMarkers("a", "b"),
			want:  []marker{{Name: "a"}, {Name: "b"}},
		},
		{
			name:  "predicate selects the one it accepts",
			entry: entryWithMarkers("a", "b"),
			opts: []data.ImplementationOption[marker]{
				data.WithTest[marker](func(in marker) bool { return in.Name == "b" }),
			},
			want: []marker{{Name: "b"}},
		},
		{
			name:  "predicate that accepts nothing selects nothing",
			entry: entryWithMarkers("a", "b"),
			opts: []data.ImplementationOption[marker]{
				data.WithTest[marker](func(_ marker) bool { return false }),
			},
			want: nil,
		},
		{
			name:  "implementations of other types are never returned",
			entry: entryWithMarkers(),
			want:  nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			got := data.GetImplementations[marker](c.entry, c.opts...)

			assert.Equal(t, c.want, got)
		})
	}
}

func TestGetImplementation(t *testing.T) {

	cases := []struct {
		name      string
		entry     *data.Entry
		opts      []data.ImplementationOption[marker]
		want      *marker
		wantPanic string
	}{
		{
			name:  "the only implementation is returned",
			entry: entryWithMarkers("a"),
			want:  &marker{Name: "a"},
		},
		{
			name:  "none is not an error",
			entry: entryWithMarkers(),
			want:  nil,
		},
		{
			name:  "a predicate disambiguates several of one type",
			entry: entryWithMarkers("a", "b"),
			opts: []data.ImplementationOption[marker]{
				data.WithTest[marker](func(in marker) bool { return in.Name == "b" }),
			},
			want: &marker{Name: "b"},
		},
		{
			name:  "several with no predicate panics, naming both types",
			entry: entryWithMarkers("a", "b"),
			wantPanic: "2 implementations of data_test.marker found on entry data_test.Dummy; " +
				"pass WithTest to select one",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			act := func() *marker { return data.GetImplementation[marker](c.entry, c.opts...) }

			if c.wantPanic != "" {
				assert.PanicsWithValue(t, c.wantPanic, func() { _ = act() })
				return
			}

			assert.Equal(t, c.want, act())
		})
	}
}
