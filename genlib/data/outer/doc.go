// Package outer generates a repository's outer proxy chain: types that
// implement a generated repository interface and wrap the flavor's
// implementation, each adding one concern — timing (Elapsed), the entity's
// own validation (Validator), an audit changelog (Diff) — and delegating
// inward.
//
// It reads the interfaces by reflection, so it runs as a second generator
// step over the compiled output of the first (genlib/data). The flavor's
// repositories go under the fx name outer.InnerName
// (data.WithRepositoryName); the chain takes them by it and provides the
// interfaces unnamed.
//
//	outer.Generate(outer.Main{
//		Dir: "../repository/outer", Package: "outer", ModuleName: "app.repository.outer",
//		Inputs: []outer.Input{
//			{Interface: reflect.TypeFor[repository.ProductRepository](), Proxies: outer.Audited},
//			{Interface: reflect.TypeFor[repository.TagRepository](), Proxies: outer.Standard},
//		},
//	})
package outer
