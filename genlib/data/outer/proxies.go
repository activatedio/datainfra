package outer

import (
	"fmt"
	"reflect"

	"github.com/dave/jennifer/jen"
	"github.com/iancoleman/strcase"

	"github.com/activatedio/datainfra/genlib/data"
)

const (
	errID      = "err"
	existingID = "existing"
	sinkField  = "diffSink"
	sinkParam  = "DiffSink"
)

// Validator calls the entity's Validate() error before Create and Update,
// and returns its error without calling inward. An entity under a
// validator proxy must have the method; generation panics otherwise.
var Validator = Proxy{Name: "validator", Method: validatorMethod}

// Elapsed logs every call's duration at debug level.
var Elapsed = Proxy{Name: "elapsed", Method: elapsedMethod}

// Diff records every write: Create, Update, Delete and DeleteEntity hand the
// changelog between the stored row and the written one to the graph's
// outer.DiffSink (outer.DefaultDiffSink, which logs, if there is none). The
// write runs first and the sink after it, so a refused write records
// nothing; the sink runs in the write's context, so a rolled-back
// transaction takes its record with it.
var Diff = Proxy{Name: "diff", Method: diffMethod, Struct: diffStruct}

// Standard is validator and elapsed, emitted elapsed → validator → inner.
var Standard = []Proxy{Validator, Elapsed}

// Audited is Standard with Diff innermost, emitted elapsed → validator →
// diff → inner: a write the validator refuses never reaches diff.
var Audited = []Proxy{Diff, Validator, Elapsed}

func returnErr() jen.Code {
	return jen.If(jen.Id(errID).Op("!=").Nil()).Block(jen.Return(jen.Id(errID)))
}

func validatorMethod(p MethodParams) []jen.Code {
	if p.Method.Name != "Create" && p.Method.Name != "Update" {
		return nil
	}
	if p.Entity == nil {
		panic(fmt.Sprintf("outer: validator on %s, which has no entity", p.Interface.Name()))
	}
	if _, ok := reflect.PointerTo(p.Entity).MethodByName("Validate"); !ok {
		panic(fmt.Sprintf("outer: validator on %s, but *%s has no Validate() error", p.Interface.Name(), p.Entity.Name()))
	}
	return append([]jen.Code{
		jen.If(jen.Id(errID).Op(":=").Id(p.Args[1]).Dot("Validate").Call(), jen.Id(errID).Op("!=").Nil()).Block(
			jen.Return(jen.Id(errID)),
		),
	}, p.Delegate...)
}

func elapsedMethod(p MethodParams) []jen.Code {
	return append([]jen.Code{
		jen.Id("start").Op(":=").Qual("time", "Now").Call(),
		jen.Defer().Func().Params().Block(
			jen.Qual("github.com/rs/zerolog/log", "Debug").Call().
				Dot("Str").Call(jen.Lit("repo"), jen.Lit(p.Interface.Name())).
				Dot("Str").Call(jen.Lit("method"), jen.Lit(p.Method.Name)).
				Dot("Dur").Call(jen.Lit("elapsed"), jen.Qual("time", "Since").Call(jen.Id("start"))).
				Dot("Msg").Call(jen.Lit("repository call")),
		).Call(),
	}, p.Delegate...)
}

// Source is the name a diff proxy records an entity under: its type in
// snake_case.
func Source(entity reflect.Type) string {
	return strcase.ToSnake(entity.Name())
}

// Redacted is the entity's fields tagged `audit:"redact"`.
func Redacted(entity reflect.Type) []string {
	var out []string
	for i := 0; i < entity.NumField(); i++ {
		if entity.Field(i).Tag.Get("audit") == "redact" {
			out = append(out, entity.Field(i).Name)
		}
	}
	return out
}

func diffMethod(p MethodParams) []jen.Code {
	if p.Entity == nil {
		panic(fmt.Sprintf("outer: diff on %s, which has no entity", p.Interface.Name()))
	}
	key := data.KeyField(p.Entity)
	if key == nil {
		panic(fmt.Sprintf("outer: diff on %s, but %s has no data:\"key\" field", p.Interface.Name(), p.Entity.Name()))
	}
	entityKey := func() jen.Code { return jen.Id("p1").Dot(key.Name) }

	metadata := func(target jen.Code) jen.Code {
		fields := []jen.Code{
			jen.Id("Source").Op(":").Lit(Source(p.Entity)).Op(","),
			jen.Id("MethodName").Op(":").Lit(p.Method.Name).Op(","),
			jen.Id("TargetID").Op(":").Qual("fmt", "Sprint").Call(target).Op(","),
		}
		if r := Redacted(p.Entity); len(r) > 0 {
			lits := make([]jen.Code, 0, len(r))
			for _, f := range r {
				lits = append(lits, jen.Lit(f))
			}
			fields = append(fields, jen.Id("Redact").Op(":").Index().String().Values(lits...).Op(","))
		}
		return jen.Qual(ImportOuter, "MethodMetadata").Block(fields...)
	}
	// sink diffs existing against to and hands it on. existing is typed as
	// any so a nil from a create or a not-found is an untyped nil to the
	// differ, and to is passed the same way.
	sink := func(to, target jen.Code) []jen.Code {
		return []jen.Code{
			jen.List(jen.Id("cl"), jen.Id(errID)).Op(":=").Qual(ImportOuter, "Diff").Call(jen.Id(existingID), to),
			returnErr(),
			jen.Return(jen.Id(receiverName).Dot(sinkField).Dot("Sink").Call(jen.Id("p0"), metadata(target), jen.Id("cl"))),
		}
	}
	// prior reads the stored row before the write destroys it.
	prior := func(k jen.Code) []jen.Code {
		return []jen.Code{
			jen.Var().Id(existingID).Any(),
			jen.List(jen.Id("found"), jen.Id(errID)).Op(":=").Id(receiverName).Dot(delegateName).Dot("FindByKey").Call(jen.Id("p0"), k),
			returnErr(),
			jen.If(jen.Id("found").Op("!=").Nil()).Block(jen.Id(existingID).Op("=").Id("found")),
		}
	}
	call := func() []jen.Code {
		return []jen.Code{
			jen.If(
				jen.Id(errID).Op(":=").Id(receiverName).Dot(delegateName).Dot(p.Method.Name).Call(jen.Id("p0"), jen.Id("p1")),
				jen.Id(errID).Op("!=").Nil(),
			).Block(jen.Return(jen.Id(errID))),
		}
	}

	var out []jen.Code
	switch p.Method.Name {
	case "Create":
		out = append(out, call()...)
		out = append(out, jen.Var().Id(existingID).Any())
		// The key is read after the write, which may be what mints it.
		out = append(out, sink(jen.Id("p1"), entityKey())...)
	case "Update":
		out = append(out, prior(entityKey())...)
		out = append(out, call()...)
		out = append(out, sink(jen.Id("p1"), entityKey())...)
	case "Delete":
		out = append(out, prior(jen.Id("p1"))...)
		out = append(out, call()...)
		out = append(out, nothingDeleted())
		out = append(out, sink(jen.Nil(), jen.Id("p1"))...)
	case "DeleteEntity":
		out = append(out, prior(entityKey())...)
		out = append(out, call()...)
		out = append(out, nothingDeleted())
		out = append(out, sink(jen.Nil(), entityKey())...)
	default:
		return nil
	}
	return out
}

// nothingDeleted ends a delete of a row that was not there: nothing
// changed, so nothing is recorded.
func nothingDeleted() jen.Code {
	return jen.If(jen.Id(existingID).Op("==").Nil()).Block(jen.Return(jen.Nil()))
}

func diffStruct(StructParams) StructResult {
	return StructResult{
		Params: []jen.Code{
			jen.Id(sinkParam).Qual(ImportOuter, "DiffSink").Tag(map[string]string{"optional": "true"}),
		},
		Fields: []jen.Code{
			jen.Id(sinkField).Qual(ImportOuter, "DiffSink"),
		},
		Assign: []jen.Code{
			jen.Id(sinkField).Op(":").Qual(ImportOuter, "SinkOrDefault").Call(jen.Id(paramsName).Dot(sinkParam)).Op(","),
		},
	}
}
