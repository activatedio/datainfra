package data

import (
	"github.com/dave/jennifer/jen"
)

// Wiring controls how generated repository constructors integrate with a
// dependency-injection framework. A nil Wiring on a flavor-specific
// DirectoryMain emits plain Go constructors with no framework imports and
// skips the index file.
//
// Wiring is intentionally flavor-agnostic. Each storage flavor (gorm, gocql,
// elasticsearch, ...) owns the list of constructor references that should be
// registered — including its own seed providers (NewDB, NewClusterConfig,
// NewTypedClient, ...) and the per-entry NewXxxRepository functions it
// generates. The Wiring decides only what to do with that list once it has
// one (fx.Module/fx.Provide, plain slice constant, etc).
type Wiring interface {
	// PrependCtorParamsFields returns fields to prepend to each generated
	// RepositoryParams struct. Typical use: an embedded fx.In tag.
	PrependCtorParamsFields() []jen.Code

	// EmitIndex writes an Index() function (or equivalent) that registers
	// the supplied constructor identifiers with the DI framework. The flavor
	// is responsible for the *contents* of provideRefs; the Wiring controls
	// *how* they get registered.
	EmitIndex(f *jen.File, provideRefs []jen.Code)
}

// RepositoryWiring is a Wiring that registers a flavor's repository
// constructors differently from its seed providers (NewDB and the like). A
// flavor's directory handler passes each repository constructor through
// WrapRepository before handing the list to EmitIndex.
type RepositoryWiring interface {
	Wiring
	WrapRepository(ctor jen.Code) jen.Code
}

// FXOption configures FXWiring.
type FXOption func(*fxWiring)

// WithRepositoryName registers every repository constructor under an fx
// name, leaving the seed providers unnamed. An outer proxy chain
// (genlib/data/outer) takes the repositories by that name — outer.InnerName —
// and provides the interfaces unnamed in their place.
func WithRepositoryName(name string) FXOption {
	return func(w *fxWiring) { w.repositoryName = name }
}

// FXWiring returns a Wiring that integrates generated repositories with
// go.uber.org/fx. moduleName is the name passed to fx.Module in the
// generated Index() function.
func FXWiring(moduleName string, opts ...FXOption) Wiring {
	w := &fxWiring{moduleName: moduleName}
	for _, o := range opts {
		o(w)
	}
	return w
}

type fxWiring struct {
	moduleName     string
	repositoryName string
}

func (w *fxWiring) WrapRepository(ctor jen.Code) jen.Code {
	if w.repositoryName == "" {
		return ctor
	}
	return jen.Qual(ImportFX, "Annotate").Call(ctor,
		jen.Qual(ImportFX, "ResultTags").Call(jen.Lit(`name:"`+w.repositoryName+`"`)))
}

func (w *fxWiring) PrependCtorParamsFields() []jen.Code {
	return []jen.Code{jen.Qual(ImportFX, "In")}
}

func (w *fxWiring) EmitIndex(f *jen.File, provideRefs []jen.Code) {
	f.Commentf("Index collects constructors for implementations in an fx module")
	f.Func().Id("Index").Params().Params(jen.Qual(ImportFX, "Option")).Block(
		jen.Return(jen.Qual(ImportFX, "Module")).Call(
			jen.Lit(w.moduleName), jen.Qual(ImportFX, "Provide").Call(provideRefs...),
		),
	)
}
