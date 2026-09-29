package outer

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/activatedio/gen"
	"github.com/dave/jennifer/jen"
	"github.com/iancoleman/strcase"

	"github.com/activatedio/datainfra/genlib/data"
)

const (
	// ImportOuter is the runtime package the generated chain calls.
	ImportOuter = "github.com/activatedio/datainfra/pkg/data/outer"

	paramsName   = "params"
	delegateName = "delegate"
	receiverName = "r"
)

// MethodParams is what a proxy's method hook is given: the interface and
// method it is generating, the plain delegation it can wrap, and the names
// of the method's parameters (p0 is always the context).
type MethodParams struct {
	Interface reflect.Type
	Entity    reflect.Type
	Method    reflect.Method
	Args      []string
	Delegate  []jen.Code
}

// StructParams is what a proxy's struct hook is given.
type StructParams struct {
	Interface reflect.Type
	Entity    reflect.Type
}

// StructResult is what a proxy adds to the chain's constructor: fields on
// the constructor's fx params, fields on its own struct, and their
// assignments when the chain is built.
type StructResult struct {
	Params []jen.Code
	Fields []jen.Code
	Assign []jen.Code
}

// Proxy is one link in a chain. Method returns a method's body, or nil for
// plain delegation. Struct, if set, adds what the proxy needs injected.
type Proxy struct {
	Name   string
	Method func(MethodParams) []jen.Code
	Struct func(StructParams) StructResult
}

// Input is one repository interface and its chain, innermost first: the
// generated call order runs from the last proxy to the first, then into
// the inner repository.
type Input struct {
	Interface reflect.Type
	Proxies   []Proxy
}

// Main generates every Input's chain into Dir, one file per proxy plus a
// constructor, and an index providing each constructor.
type Main struct {
	Dir     string
	Package string
	// ModuleName names the fx.Module the index returns.
	ModuleName string
	Inputs     []Input
}

// Generate writes the chains.
func Generate(m Main) {
	ctors := make([]jen.Code, 0, len(m.Inputs))
	for _, in := range m.Inputs {
		if in.Interface.Kind() != reflect.Interface {
			panic(fmt.Sprintf("outer: %s is not an interface", in.Interface))
		}
		if len(in.Proxies) == 0 {
			panic(fmt.Sprintf("outer: %s has no proxies", in.Interface.Name()))
		}
		generateChain(m, in)
		ctors = append(ctors, jen.Line().Id("New"+in.Interface.Name()))
	}
	gen.WithFile(m.Package, filepath.Join(m.Dir, "index_gen.go"), func(f *jen.File) {
		f.Comment("Index provides every repository's outer chain in place of the inner")
		f.Comment("repositories, which it takes by the name " + `"` + "inner" + `"` + ".")
		f.Func().Id("Index").Params().Qual(data.ImportFX, "Option").Block(
			jen.Return(jen.Qual(data.ImportFX, "Module").Call(
				jen.Lit(m.ModuleName),
				jen.Qual(data.ImportFX, "Provide").Call(append(ctors, jen.Line())...),
			)),
		)
	})
}

func baseName(t reflect.Type) string {
	return strcase.ToSnake(strings.TrimSuffix(t.Name(), "Repository"))
}

func implName(t reflect.Type, p Proxy) string {
	return strcase.ToLowerCamel(t.Name()) + strcase.ToCamel(p.Name)
}

func generateChain(m Main, in Input) {
	t := in.Interface
	entity := EntityType(t)
	var ctorParams []jen.Code

	for _, p := range in.Proxies {
		var sr StructResult
		if p.Struct != nil {
			sr = p.Struct(StructParams{Interface: t, Entity: entity})
			ctorParams = append(ctorParams, sr.Params...)
		}
		name := implName(t, p)
		gen.WithFile(m.Package, filepath.Join(m.Dir, fmt.Sprintf("%s_%s_gen.go", baseName(t), p.Name)), func(f *jen.File) {
			fields := append([]jen.Code{jen.Id(delegateName).Add(typeCode(t))}, sr.Fields...)
			f.Commentf("%s is the %s proxy over %s.", name, p.Name, t.Name())
			f.Type().Id(name).Struct(fields...)
			for i := 0; i < t.NumMethod(); i++ {
				generateMethod(f, t, entity, p, name, t.Method(i))
			}
		})
	}

	params := t.Name() + "Params"
	gen.WithFile(m.Package, filepath.Join(m.Dir, baseName(t)+"_gen.go"), func(f *jen.File) {
		fields := append([]jen.Code{
			jen.Qual(data.ImportFX, "In"),
			jen.Id("Delegate").Add(typeCode(t)).Tag(map[string]string{"name": "inner"}),
		}, ctorParams...)
		f.Commentf("%s are the parameters of New%s.", params, t.Name())
		f.Type().Id(params).Struct(fields...)
		f.Commentf("New%s is %s's outer chain over the inner repository.", t.Name(), t.Name())
		f.Func().Id("New" + t.Name()).Params(jen.Id(paramsName).Id(params)).Add(typeCode(t)).Block(
			jen.Return(chain(t, entity, in.Proxies)),
		)
	})
}

// chain builds the literal for the proxies, outermost (the last) first.
func chain(t, entity reflect.Type, proxies []Proxy) jen.Code {
	p := proxies[len(proxies)-1]
	var inner jen.Code
	if len(proxies) == 1 {
		inner = jen.Id(paramsName).Dot("Delegate")
	} else {
		inner = chain(t, entity, proxies[:len(proxies)-1])
	}
	fields := []jen.Code{jen.Id(delegateName).Op(":").Add(inner).Op(",")}
	if p.Struct != nil {
		fields = append(fields, p.Struct(StructParams{Interface: t, Entity: entity}).Assign...)
	}
	return jen.Op("&").Id(implName(t, p)).Block(fields...)
}

func generateMethod(f *jen.File, t, entity reflect.Type, p Proxy, name string, m reflect.Method) {
	var params, results, args []jen.Code
	var names []string
	variadic := m.Type.IsVariadic()
	for i := 0; i < m.Type.NumIn(); i++ {
		id := fmt.Sprintf("p%d", i)
		names = append(names, id)
		pt := m.Type.In(i)
		if variadic && i == m.Type.NumIn()-1 {
			params = append(params, jen.Id(id).Op("...").Add(typeCode(pt.Elem())))
			args = append(args, jen.Id(id).Op("..."))
			continue
		}
		params = append(params, jen.Id(id).Add(typeCode(pt)))
		args = append(args, jen.Id(id))
	}
	for i := 0; i < m.Type.NumOut(); i++ {
		results = append(results, typeCode(m.Type.Out(i)))
	}
	delegate := []jen.Code{jen.Return(jen.Id(receiverName).Dot(delegateName).Dot(m.Name).Call(args...))}
	body := p.Method(MethodParams{Interface: t, Entity: entity, Method: m, Args: names, Delegate: delegate})
	if body == nil {
		body = delegate
	}
	f.Commentf("%s implements %s.", m.Name, t.Name())
	f.Func().Params(jen.Id(receiverName).Op("*").Id(name)).Id(m.Name).Params(params...).Params(results...).Block(body...).Line()
}

// EntityType is the entity a repository interface is over, read off Create's
// entity parameter; nil for an interface with no Create.
func EntityType(t reflect.Type) reflect.Type {
	m, ok := t.MethodByName("Create")
	if !ok || m.Type.NumIn() < 2 {
		return nil
	}
	e := m.Type.In(1)
	for e.Kind() == reflect.Pointer {
		e = e.Elem()
	}
	if e.Kind() != reflect.Struct {
		return nil
	}
	return e
}
