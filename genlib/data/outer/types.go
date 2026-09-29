package outer

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/dave/jennifer/jen"
)

// typeCode renders a reflected type as jen code, qualified by import path so
// jen writes the imports. It handles what repository interfaces carry:
// pointers, slices, maps, named types and instantiated generics, whose
// reflected name spells its type arguments with their full import paths —
// data.List[*github.com/acme/model.Product].
func typeCode(t reflect.Type) jen.Code {
	s := &jen.Statement{}
	writeType(t, s)
	return s
}

func writeType(t reflect.Type, s *jen.Statement) {
	switch t.Kind() {
	case reflect.Pointer:
		s.Op("*")
		writeType(t.Elem(), s)
		return
	case reflect.Slice:
		if t.Name() == "" {
			s.Index()
			writeType(t.Elem(), s)
			return
		}
	case reflect.Map:
		if t.Name() == "" {
			k := &jen.Statement{}
			writeType(t.Key(), k)
			s.Map(k)
			writeType(t.Elem(), s)
			return
		}
	case reflect.Interface:
		if t.Name() == "" {
			if t.NumMethod() == 0 {
				s.Any()
				return
			}
			panic("outer: an unnamed interface type with methods cannot be rendered: " + t.String())
		}
	default:
	}
	if t.Name() == "" {
		panic("outer: an unnamed " + t.Kind().String() + " type cannot be rendered: " + t.String())
	}
	writeNamed(parseTypeName(t.PkgPath(), t.Name()), s)
}

// named is a type name as reflection spells it, split into its package,
// name and type arguments.
type named struct {
	ptr, slice bool
	pkg, name  string
	args       []named
}

var qualifiedRegexp = regexp.MustCompile(`^(.+)\.([A-Za-z0-9_]+)$`)

// parseTypeName splits a reflected name. pkg is the type's own package, which
// reflection gives separately for the outer type and inline for each type
// argument.
func parseTypeName(pkg, name string) named {
	out := named{pkg: pkg, name: name}
	open := strings.IndexByte(name, '[')
	if open < 0 {
		return out
	}
	out.name = name[:open]
	for _, a := range splitArgs(name[open+1 : len(name)-1]) {
		out.args = append(out.args, parseArg(a))
	}
	return out
}

func parseArg(in string) named {
	var out named
	for {
		switch {
		case strings.HasPrefix(in, "[]"):
			out.slice, in = true, in[2:]
			continue
		case strings.HasPrefix(in, "*"):
			out.ptr, in = true, in[1:]
			continue
		}
		break
	}
	head := in
	if open := strings.IndexByte(in, '['); open >= 0 {
		head = in[:open]
	}
	pkg, name := "", head
	if m := qualifiedRegexp.FindStringSubmatch(head); m != nil {
		pkg, name = m[1], m[2]
	}
	inner := parseTypeName(pkg, name+in[len(head):])
	inner.ptr, inner.slice = out.ptr, out.slice
	return inner
}

// splitArgs splits a type-argument list at its top-level commas.
func splitArgs(in string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range in {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, in[start:i])
				start = i + 1
			}
		}
	}
	return append(out, in[start:])
}

func writeNamed(n named, s *jen.Statement) {
	if n.slice {
		s.Index()
	}
	if n.ptr {
		s.Op("*")
	}
	if n.pkg == "" {
		s.Id(n.name)
	} else {
		s.Qual(n.pkg, n.name)
	}
	if len(n.args) > 0 {
		args := make([]jen.Code, 0, len(n.args))
		for _, a := range n.args {
			as := &jen.Statement{}
			writeNamed(a, as)
			args = append(args, as)
		}
		s.Types(args...)
	}
}
