package data

import (
	"fmt"
	"reflect"

	"github.com/activatedio/datainfra/genlib"
	"github.com/dave/jennifer/jen"
)

// ImportThis is a package import path used for internal references within the codebase.
var (
	ImportThis = "github.com/activatedio/datainfra/pkg/data"
)

// Types represents a collection of Entries along with the associated package information.
type Types struct {
	Package string
	Entries []Entry
}

// GetPackage returns the package name associated with the Types instance.
func (t *Types) GetPackage() string {
	return t.Package
}

// Crud represents a type encapsulating a set of operations for performing basic CRUD (Create, Read, Update, Delete) functionality.
type Crud struct {
	Operations *genlib.Set[Operation]
}

// Search defines a type used for implementing search-related logic and behavior in the system.
type Search struct {
}

// Associate represents a type that holds a child type as a reflection of its associated entity.
type Associate struct {
	ChildType reflect.Type
}

// FilterKeys represents a type that defines filtering logic for a specific set of keys within a data structure or collection.
type FilterKeys struct {
}

// ListByAssociatedKey specifies a type associated with another entity for relation-based operations or queries.
type ListByAssociatedKey struct {
	AssociatedType reflect.Type
}

// InterfaceMethods represents a type used for generating method definitions based on the associated `Entry` metadata.
type InterfaceMethods struct {
	Entry *Entry
}

// addBaseHandlers adds a file handler for processing repository interfaces based on provided type entries.
// It registers the handler using a key generated for the *Types type and returns the updated HandlerEntries instance.
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

// addCrudHandlers adds CRUD operation handlers to the given HandlerEntries if the InterfaceMethods has a Crud implementation.
// It registers handlers for operations such as FindByKey, ExistsByKey, ListAll, Create, Update, Delete, and DeleteEntity.
func addCrudHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[Crud](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

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
					IDError,
				)).Add(jen.Id("ExistsByKey").Params(
					QualCtx,
					jh.GenerateKeyCode(""),
				).Params(
					jen.Bool(),
					IDError,
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

// addSearchHandlers registers search-related statement handlers into the provided HandlerEntries and returns the updated instance.
func addSearchHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[Search](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

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

// addAssociateHandlers registers a StatementHandler for handling Associate implementations in the provided HandlerEntries.
func addAssociateHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[Associate](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

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

// addFilterKeysHandlers registers a statement handler that binds the FilterKeys operation to entries implementing the FilterKeys interface.
// The function checks if an entry supports the FilterKeys implementation and generates the corresponding handler logic.
// Returns the updated HandlerEntries with the newly added statement handler for chaining.
func addFilterKeysHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[FilterKeys](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

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

// addListByAssociatedKeyHandlers appends a statement handler to manage ListByAssociatedKey operations for InterfaceMethods.
// It sets up handlers to generate code for listing entries by their associated key using specific type metadata.
func addListByAssociatedKeyHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*InterfaceMethods](func(in *InterfaceMethods) bool {
		return HasImplementation[ListByAssociatedKey](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

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

// NewDataRegistry initializes and returns a new genlib.Registry instance with predefined handler entries for various operations.
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
