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

// FXWiring returns a Wiring that integrates generated repositories with
// go.uber.org/fx. moduleName is the name passed to fx.Module in the
// generated Index() function.
func FXWiring(moduleName string) Wiring {
	return &fxWiring{moduleName: moduleName}
}

type fxWiring struct {
	moduleName string
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
