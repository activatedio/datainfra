package data

import (
	"fmt"
	"reflect"

	"github.com/activatedio/datainfra/genlib"
	"github.com/dave/jennifer/jen"
)

// ImportThis defines the import path for the data infrastructure package used in the application.
var (
	ImportThis = "github.com/activatedio/datainfra/pkg/data"
)

// Types represents a collection of data types, consisting of a package name and a list of entries describing the types.
type Types struct {
	Package string
	Entries []Entry
}

// GetPackage retrieves the package name associated with the Types instance.
func (t *Types) GetPackage() string {
	return t.Package
}

// Crud represents a marker type used to define CRUD-related operations
type Crud struct {
	Operations *genlib.Set[Operation]
}

// Search represents a marker type used to define search-related operations in data handling systems.
type Search struct {
}

// Associate defines a type used for managing and linking associated entities or data in a structured system.
type Associate struct {
	ChildType reflect.Type
}

type FilterKeys struct {
}

type ListByAssociatedKey struct {
	AssociatedType reflect.Type
}

// InterfaceMethods represents metadata for generating interface methods for a specific entry type.
type InterfaceMethods struct {
	Entry *Entry
}

func addBaseHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {
	return he.AddFileHandler(genlib.NewKey[*Types](), func(f *jen.File, r genlib.Registry, entry any) {

		t := entry.(*Types)
		ds := t.Entries

		for _, d := range ds {

			jh := d.GetJenHelper()

			if jh.keyStmt != nil {
				f.Add(jh.keyStmt)
			}

			f.Commentf("%s is a repository for the type %s", jh.InterfaceName,
				d.Type.Name()).Line().Type().Id(jh.InterfaceName).Interface(
				*r.BuildStatement(&jen.Statement{}, &InterfaceMethods{
					Entry: &d,
				})...,
			)
		}

	})
}

func addCrudHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[Crud](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		i := entry.(*InterfaceMethods)
		d := i.Entry

		jh := d.GetJenHelper()

		c := GetImplementation[Crud](d)

		for _, op := range c.Operations.All() {
			switch op {
			case OperationFindByKey:
				s.Add(jen.Id("FindByKey").Params(
					QualCtx,
					jh.GenerateKeyCode(""),
				).Params(
					jen.Op("*").Add(jh.StructType),
					IdError,
				)).Add(jen.Id("ExistsByKey").Params(
					QualCtx,
					jh.GenerateKeyCode(""),
				).Params(
					jen.Bool(),
					IdError,
				))
			case OperationList:
				s.Add(jen.Id("ListAll").Params(
					QualCtx, jen.Qual(ImportThis, "ListParams")).Params(
					jen.Op("*").Qual(ImportThis, "List").Types(
						jen.Op("*").Add(jh.StructType),
					),
					jen.Error(),
				))
			case OperationCreate:
				s.Add(jen.Id("Create").Params(
					QualCtx, jen.Op("*").Add(jh.StructType)).Params(
					jen.Error(),
				))
			case OperationUpdate:
				s.Add(jen.Id("Update").Params(
					QualCtx, jen.Op("*").Add(jh.StructType)).Params(
					jen.Error(),
				))
			case OperationDelete:
				s.Add(jen.Id("Delete").Params(
					QualCtx, jh.GenerateKeyCode("")).Params(
					jen.Error(),
				))
				s.Add(jen.Id("DeleteEntity").Params(
					QualCtx, jen.Op("*").Add(jh.StructType)).Params(
					jen.Error(),
				))
			}
		}

		return s
	})

}

func addSearchHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[Search](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		i := entry.(*InterfaceMethods)

		jh := i.Entry.GetJenHelper()

		return s.Add(
			jen.Id("Search").Params(
				jen.Id("ctx").Add(QualCtx),
				jen.Id("criteria").Op("[]*").Qual(ImportThis, "SearchPredicate"),
				jen.Id("params").Op("*").Qual(ImportThis, "PageParams"),
			).Params(jen.Op("*").Qual(ImportThis, "List").Types(jen.Op("*").Qual(ImportThis, "SearchResult").Types(jen.Op("*").Add(jh.StructType))), jen.Error())).Add(
			jen.Id("GetSearchPredicates").Params(QualCtx).Params(jen.Op("[]*").Qual(ImportThis, "SearchPredicateDescriptor"), jen.Error()))

	})

}

func addAssociateHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[Associate](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		i := entry.(*InterfaceMethods)

		a := GetImplementation[Associate](i.Entry)

		jh := i.Entry.GetJenHelper()

		_e := &Entry{
			Type: a.ChildType,
		}

		jhc := _e.GetJenHelper()

		ckc := jhc.GenerateKeyCode("")

		return s.Add(jen.Id(fmt.Sprintf("Associate%s", Pl.Plural(jhc.StructName))).Params(jen.Id("ctx").Add(QualCtx), jen.Id("key").Add(jh.GenerateKeyCode("")), jen.Id("add").Index().Add(ckc), jen.Id("remove").Index().Add(ckc)).Params(jen.Error()))

	})

}

func addFilterKeysHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[FilterKeys](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		i := entry.(*InterfaceMethods)

		jh := i.Entry.GetJenHelper()

		kc := jh.GenerateKeyCode("")

		return s.Add(
			jen.Id("FilterKeys").Params(
				jen.Id("ctx").Add(QualCtx),
				jen.Id("keys").Index().Add(kc),
			).Params(jen.Index().Add(kc), jen.Error()))

	})

}

func addListByAssociatedKeyHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[ListByAssociatedKey](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		i := entry.(*InterfaceMethods)

		a := GetImplementation[ListByAssociatedKey](i.Entry)

		jh := i.Entry.GetJenHelper()

		_e := &Entry{
			Type: a.AssociatedType,
		}

		jha := _e.GetJenHelper()

		cka := jha.GenerateKeyCode("")

		return s.Add(jen.Id(fmt.Sprintf("ListBy%s", jha.StructName)).Params(
			jen.Id("ctx").Add(QualCtx),
			jen.Id("key").Add(cka),
			jen.Id("params").Qual(ImportThis, "ListParams"),
		).Params(
			jen.Op("*").Qual(ImportThis, "List").Types(
				jen.Op("*").Add(jh.StructType),
			),
			jen.Error(),
		))

	})
}

// NewDataRegistry initializes a new data registry with custom handler entries for files, statements, and interface methods.
func NewDataRegistry() genlib.Registry {

	he := genlib.NewHandlerEntries()
	he = addBaseHandlers(he)
	he = addCrudHandlers(he)
	he = addSearchHandlers(he)
	he = addAssociateHandlers(he)
	he = addFilterKeysHandlers(he)
	he = addListByAssociatedKeyHandlers(he)

	return genlib.NewRegistry().WithHandlerEntries(he)

}
