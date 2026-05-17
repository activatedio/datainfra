// Package main contains the main method for generation
package main

import (
	"reflect"

	"github.com/activatedio/gen"
	"github.com/dave/jennifer/jen"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/genlib/data"
	"github.com/activatedio/datainfra/genlib/data/gorm"
	data2 "github.com/activatedio/datainfra/pkg/data"
)

//go:generate go run .

func main() {

	ds := []data.Entry{
		{
			Type: reflect.TypeFor[model.Location](),
			Implementations: []any{
				data.Crud{
					Operations: data.OperationsCrud,
				},
			},
		},
		{
			Type: reflect.TypeFor[model.Category](),
			Implementations: []any{
				data.Crud{
					Operations: data.OperationsCrud,
				},
				data.FilterKeys{},
				data.ListByAssociatedKey{
					AssociatedType: reflect.TypeFor[model.Product](),
					Reversed:       true,
				},
			},
		},
		{
			Type: reflect.TypeFor[model.Product](),
			Implementations: []any{
				data.Crud{
					Operations: data.OperationsCrud,
				},
				data.Search{},
				data.Associate{
					ChildType: reflect.TypeFor[model.Category](),
				},
				data.ListByAssociatedKey{
					AssociatedType: reflect.TypeFor[model.Category](),
				},
				gorm.Search{
					Bindings: []gorm.SearchBinding{
						{
							Name:      "@keywords",
							Label:     "Keywords",
							Virtual:   true,
							Operators: []data2.SearchOperator{data2.SearchOperatorStringMatch},
							Binder: gorm.DialectBinderCall(map[string]jen.Code{
								"postgres": gorm.PostgresKeywordsBinderCall("full_text"),
								"sqlite":   gorm.LikeBinderCall("description"),
							}),
						},
						{
							Name:      "@query",
							Label:     "Query",
							Virtual:   true,
							Operators: []data2.SearchOperator{data2.SearchOperatorStringMatch},
							Binder:    gorm.LikeBinderCall("description"),
						},
					},
				},
			},
		},
		{
			Type: reflect.TypeFor[model.Theme](),
			Implementations: []any{
				data.Crud{
					Operations: data.OperationsCrud,
				},
				gorm.Implementation{
					TableName:        "themes2",
					ContextScopeCode: jen.Id("WithTenantScope").Call(),
				},
			},
		},
	}

	data.NewDataRegistry().RunFilePathHandler("../repository/types.go", &data.Types{
		Package: "repository",
		Entries: ds,
	})

	gorm.NewDataRegistry().WithHandlerEntries(gen.NewHandlerEntries().AddStatementHandler(
		gen.NewKeyWithTest[*gorm.InternalFields](func(in *gorm.InternalFields) bool {
			return in.Entry.Type == reflect.TypeFor[model.Theme]()
		}), func(s *jen.Statement, _ gen.Registry, _ any) *jen.Statement {
			return s.Add(jen.Id("TenantID").String())
		},
	).AddFileHandler(
		gen.NewKeyWithTest[*gorm.InternalFunctions](func(in *gorm.InternalFunctions) bool {
			return in.Entry.Type == reflect.TypeFor[model.Theme]()
		}), func(f *jen.File, _ gen.Registry, _ any) {
			f.Comment("SetTenantID sets the tenant ID for the ThemeInternal")
			f.Func().Params(jen.Id("r").Op("*").Id("ThemeInternal")).Id("SetTenantID").Params(
				jen.Id("id").String(),
			).Block(
				jen.Id("r").Dot("TenantID").Op("=").Id("id"),
			)
		},
	)).RunDirectoryPathHandler("../repository/gorm", &gorm.DirectoryMain{
		InterfaceImport: "github.com/activatedio/datainfra/examples/data/repository",
		Package:         "gorm",
		Entries:         ds,
		Wiring:          data.FXWiring("example.data.gorm"),
	})

}
