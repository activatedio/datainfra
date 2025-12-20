package gorm

import (
	"fmt"
	"path/filepath"

	"github.com/activatedio/datainfra/genlib"
	"github.com/activatedio/datainfra/genlib/data"
	"github.com/dave/jennifer/jen"
	"github.com/iancoleman/strcase"
)

// ImportThis defines the import path for the gorm package utilized by the data infrastructure library.
var (
	ImportThis = "github.com/activatedio/datainfra/pkg/data/gorm"
)

// DirectoryMain represents a configuration for generating files and directories containing code based on supplied entries.
// Package defines the package name for generated files.
// InterfaceImport specifies the import path of the interfaces used by the entries.
// GenerateIndex determines whether an index file should be generated.
// IndexModule defines the name of the fx module for the generated index.
// Entries is a collection of data Entry objects to process and use for code generation.
type DirectoryMain struct {
	Package         string
	InterfaceImport string
	GenerateIndex   bool
	IndexModule     string
	Entries         []data.Entry
}

// IndexMain represents a collection of entries grouped under an index module, primarily used for fx module generation.
// IndexModule refers to the module name used for fx injection.
// Entries contains a list of data.Entry elements to be processed.
type IndexMain struct {
	IndexModule string
	Entries     []data.Entry
}

// FileMain serves as a descriptor to facilitate code generation for a specific type using metadata from a data.Entry.
// Entry holds type-specific metadata and related operations for code generation.
// InterfaceImport specifies the import path for the target interface in the generated code.
type FileMain struct {
	Entry           *data.Entry
	InterfaceImport string
}

// InternalSuperFields is a struct that contains a reference to a data.Entry, used for managing type-specific metadata.
type InternalSuperFields struct {
	Entry *data.Entry
}

// InternalFields is an empty struct used as a marker or placeholder within the codebase.
type InternalFields struct {
	Entry *data.Entry
}

// InternalFunctions represents a set of functions used internally for processing specific tasks or transformations.
type InternalFunctions struct {
	Entry *data.Entry
}

// ImplFields represents the key fields required for generating implementations tied to a data entry.
type ImplFields struct {
	Entry           *data.Entry
	InterfaceImport string
}

// ImplFieldAssignments represents a structure used for assigning implementation-specific fields in generated code.
type ImplFieldAssignments struct {
	Entry           *data.Entry
	InterfaceImport string
}

// CtorParamsFields represents the fields required to construct certain implementations, including metadata and imports.
type CtorParamsFields struct {
	Entry           *data.Entry
	InterfaceImport string
}

// Ctor represents a constructor wrapper containing a reference to a data entry for further processing or template generation.
type Ctor struct {
	Entry *data.Entry
}

// TemplateFields defines a structured type used for mapping data between internal and external representations.
type TemplateFields struct{}

// CrudTemplateParamsField is a type used to define parameters for CRUD template configurations within a registry or handler.
type CrudTemplateParamsField struct{}

// addBaseHandlers configures and registers default directory, file, and statement handlers in the provided HandlerEntries.
func addBaseHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddDirectoryHandler(genlib.NewKey[*DirectoryMain](), func(dirPath string, r genlib.Registry, entry any) {

		m := entry.(*DirectoryMain)

		for _, e := range m.Entries {
			genlib.WithFile(m.Package, filepath.Join(dirPath, fmt.Sprintf("%s_gen.go", strcase.ToSnake(e.Type.Name()))), func(file *jen.File) {
				r.RunFileHandler(file, &FileMain{
					InterfaceImport: m.InterfaceImport,
					Entry:           &e,
				})
			})
		}

		if m.GenerateIndex {
			genlib.WithFile(m.Package, filepath.Join(dirPath, "index_gen.go"), func(file *jen.File) {
				r.RunFileHandler(file, &IndexMain{
					IndexModule: m.IndexModule,
					Entries:     m.Entries,
				})
			})
		}

	}).AddFileHandler(genlib.NewKey[*IndexMain](), func(f *jen.File, _ genlib.Registry, entry any) {

		im := entry.(*IndexMain)

		opts := &jen.Statement{}

		opts.Add(
			jen.Qual(ImportThis, "NewDB"),
			jen.Qual(ImportThis, "NewContextBuilder"),
		)

		for _, d := range im.Entries {
			opts = opts.Add(jen.Id(fmt.Sprintf("New%sRepository", d.Type.Name())))
		}

		f.Commentf("Index collects constructors for implementations in an fx module")
		f.Func().Id("Index").Params().Params(jen.Qual(data.ImportFX, "Option")).Block(
			jen.Return(jen.Qual(data.ImportFX, "Module")).Call(
				jen.Lit(im.IndexModule), jen.Qual(data.ImportFX, "Provide").Call(*opts...),
			),
		)

	}).AddFileHandler(genlib.NewKey[*FileMain](), func(f *jen.File, r genlib.Registry, entry any) {

		fm := entry.(*FileMain)
		d := fm.Entry

		jh := d.GetJenHelper()
		internalName := jh.StructName + "Internal"
		implName := strcase.ToLowerCamel(jh.StructName) + "RepositoryImpl"

		fs := *r.BuildStatement(&jen.Statement{}, &InternalSuperFields{
			Entry: d,
		})
		fs = append(fs, *r.BuildStatement(&jen.Statement{}, &InternalFields{
			Entry: d,
		})...)
		f.Commentf("%s is the internal representation of %s", internalName, jh.StructName)
		f.Type().Id(internalName).Struct(fs...)
		r.RunFileHandler(f, &InternalFunctions{
			Entry: d,
		})

		implFields := &jen.Statement{}
		implFields.Add(jen.Id("Template").Qual(ImportThis, "MappingTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName)))
		implFields = r.BuildStatement(implFields, &ImplFields{
			Entry:           d,
			InterfaceImport: fm.InterfaceImport,
		})
		f.Commentf("%s is the implementation of %sRepository", implName, jh.StructName)
		f.Type().Id(implName).Struct(*implFields...)

		paramsType := fmt.Sprintf("%sRepositoryParams", jh.StructName)

		cpfStmt := &jen.Statement{}
		// TODO - Add in FX decorators
		cpfStmt.Add(jen.Qual(data.ImportFX, "In"))
		cpfStmt.Add(*r.BuildStatement(&jen.Statement{}, &CtorParamsFields{
			Entry:           d,
			InterfaceImport: fm.InterfaceImport,
		})...)

		f.Commentf("%s are the parameters for %sRepository", paramsType, jh.StructName)
		f.Type().Id(paramsType).Struct(*cpfStmt...)

		ctor := &jen.Statement{}
		r.BuildStatement(ctor, &Ctor{
			Entry: d,
		})

		ctor.Add(jen.Return(jen.Op("&").Qual("", implName).Block(
			*r.BuildStatement(&jen.Statement{}, &ImplFieldAssignments{
				Entry:           d,
				InterfaceImport: fm.InterfaceImport,
			})...,
		)))

		paramsID := "params"

		if len(*cpfStmt) == 1 {
			paramsID = ""
		}

		f.Commentf("New%sRepository creates a new %sRepository", jh.StructName, jh.StructName)
		f.Func().Id(fmt.Sprintf("New%sRepository", jh.StructName)).Params(
			jen.Id(paramsID).Id(paramsType),
		).Qual(fm.InterfaceImport, jh.InterfaceName).Block(*ctor...).Line()
	}).AddStatementHandler(genlib.NewKey[*InternalSuperFields](), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		fm := entry.(*InternalSuperFields)
		d := fm.Entry
		jh := d.GetJenHelper()
		return s.Add(jen.Op("*").Add(jh.StructType))

	}).AddStatementHandler(genlib.NewKey[*Ctor](), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		fm := entry.(*Ctor)
		d := fm.Entry
		jh := GetGormJenHelper(d)

		internalName := jh.StructName + "Internal"
		tmplStmt := &jen.Statement{}
		if jh.ContextScopeCode != nil {
			tmplStmt.Add(jen.Id("ContextScope").Op(":").Add(jh.ContextScopeCode).Op(","))
		}
		tmplStmt.Add(jen.Id("Table").Op(":").Lit(jh.TableName).Op(","))
		tmplStmt.Add(jen.Id("ToInternal").Op(":").Func().Params(
			jen.Id("m").Op("*").Add(jh.StructType),
		).Op("*").Id(internalName).Block(
			jen.Return(jen.Op("&").Id(internalName).Block(
				jen.Id(jh.StructName).Op(":").Id("m").Op(","),
			)),
		).Op(","))
		tmplStmt.Add(jen.Id("FromInternal").Op(":").Func().Params(
			jen.Id("m").Op("*").Id(internalName),
		).Op("*").Add(jh.StructType).Block(
			jen.Return(jen.Id("m").Op(".").Id(jh.StructName)),
		).Op(","))

		return s.Add(jen.Id("template").Op(":=").Qual(ImportThis, "NewMappingTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Call(jen.Qual(ImportThis, "MappingTemplateParams").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Block(*tmplStmt...)))
	}).AddStatementHandler(genlib.NewKey[*ImplFieldAssignments](), func(s *jen.Statement, _ genlib.Registry, _ any) *jen.Statement {
		return s.Add(jen.Id("Template").Op(":").Id("template").Op(","))
	})
}

// addCrudHandlers adds CRUD-specific statement handlers to the provided HandlerEntries based on certain entry conditions.
// It registers handlers for CRUD template generation and field assignments if CRUD operations are applicable.
func addCrudHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.Crud](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFields)
		d := _if.Entry
		jh := d.GetJenHelper()

		c := data.GetImplementation[data.Crud](d)

		// Determine if we have any crud operations
		if c.Operations.Intersect(data.OperationsCrud).Len() == 0 {
			// Short circuit
			return s
		}

		return s.Add(jen.Qual(data.ImportThis, "CrudTemplate").Types(
			jen.Op("*").Add(jh.StructType),
			jh.GenerateKeyCode(_if.InterfaceImport),
		))

	}).AddStatementHandler(genlib.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.Crud](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := GetGormJenHelper(d)

		c := data.GetImplementation[data.Crud](d)

		if c.Operations.Intersect(data.OperationsCrud).Len() == 0 {
			// Short circuit
			return s
		}

		internalName := jh.StructName + "Internal"

		crudParamsFields := r.BuildStatement(&jen.Statement{}, &CrudTemplateParamsField{})

		if len(jh.KeyFields) == 1 {
			crudParamsFields.Add(jen.Id("FindBuilder").Op(":").Qual(ImportThis, "SingleFindBuilder").Types(
				jh.GenerateKeyCode(_if.InterfaceImport)).Params(jen.Lit(fmt.Sprintf("%s.%s", jh.TableName, jh.KeyFields[0].Name))).Op(","))
		}

		return s.Add(jen.Id("CrudTemplate").Op(":").Qual(ImportThis, "NewMappingCrudTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName), jh.GenerateKeyCode(_if.InterfaceImport),
		).Params(jen.Qual(ImportThis, "MappingCrudTemplateImplOptions").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName), jh.GenerateKeyCode(_if.InterfaceImport),
		).Block(
			jen.Id("Template").Op(":").Id("template").Op(","),
			crudParamsFields,
		)).Op(","))

	})

}

// addSearchHandlers registers statement handlers for search implementations in handler entries and returns the updated instance.
func addSearchHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.Search](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFields)
		d := _if.Entry
		jh := d.GetJenHelper()

		return s.Add(jen.Qual(data.ImportThis, "SearchTemplate").Types(
			jen.Op("*").Add(jh.StructType),
		))

	}).AddStatementHandler(genlib.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.Search](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := d.GetJenHelper()

		srch := data.GetImplementation[data.Search](d)

		var predicates *jen.Statement

		if len(srch.Predicates) == 0 {
			predicates = jen.Nil()
		} else {
			predicates = srch.Predicates.Generate()
		}

		internalName := jh.StructName + "Internal"
		return s.Add(jen.Id("SearchTemplate").Op(":").Qual(ImportThis, "NewMappingSearchTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Params(jen.Qual(ImportThis, "MappingSearchTemplateParams").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Block(
			jen.Id("Template").Op(":").Id("template").Op(","),
			jen.Id("SearchPredicates").Op(":").Add(predicates).Op(","),
		)).Op(","))

	})

}

// addAssociateHandlers adds handlers to facilitate the management of associate relationships between data entities.
// It updates the given HandlerEntries by registering statement and file handlers for entries with 'Associate' implementations.
// Returns the updated HandlerEntries instance.
func addAssociateHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	type helper struct {
		parentHelper JenHelper
		childHelper  JenHelper
	}

	toHelper := func(e *data.Entry) helper {

		a := data.GetImplementation[data.Associate](e)

		_e := &data.Entry{
			Type: a.ChildType,
		}

		return helper{
			parentHelper: GetGormJenHelper(e),
			childHelper:  GetGormJenHelper(_e),
		}

	}

	return he.AddStatementHandler(genlib.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		f := entry.(*ImplFields)
		h := toHelper(f.Entry)

		return s.Add(jen.Id(fmt.Sprintf("%sRepository", strcase.ToLowerCamel(h.childHelper.StructName))).Qual(f.InterfaceImport, h.childHelper.InterfaceName))

	}).AddStatementHandler(genlib.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		f := entry.(*ImplFieldAssignments)
		h := toHelper(f.Entry)

		return s.Add(jen.Id(fmt.Sprintf("%sRepository", strcase.ToLowerCamel(h.childHelper.StructName))).Op(":").
			Id("params").
			Dot(fmt.Sprintf("%sRepository", h.childHelper.StructName)).Op(","))

	}).AddStatementHandler(genlib.NewKeyWithTest[*CtorParamsFields](func(in *CtorParamsFields) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		f := entry.(*CtorParamsFields)
		h := toHelper(f.Entry)

		return s.Add(jen.Id(fmt.Sprintf("%sRepository", h.childHelper.StructName)).
			Qual(f.InterfaceImport, fmt.Sprintf("%sRepository", h.childHelper.StructName)))

	}).AddFileHandler(genlib.NewKeyWithTest[*FileMain](func(in *FileMain) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(f *jen.File, _ genlib.Registry, entry any) {

		fm := entry.(*FileMain)
		h := toHelper(fm.Entry)

		kc := h.parentHelper.GenerateKeyCode("")
		ckc := h.childHelper.GenerateKeyCode("")

		implName := strcase.ToLowerCamel(h.parentHelper.StructName) + "RepositoryImpl"
		receiverID := func() *jen.Statement { return jen.Id("r") }
		keyID := func() *jen.Statement { return jen.Id("key") }
		addID := func() *jen.Statement { return jen.Id("add") }
		removeID := func() *jen.Statement { return jen.Id("remove") }
		ctxID := func() *jen.Statement { return jen.Id("ctx") }

		if len(h.parentHelper.Keys) != 1 {
			panic(fmt.Sprintf("Associate only supports a single key, found %d", len(h.parentHelper.Keys)))
		}
		if len(h.childHelper.Keys) != 1 {
			panic(fmt.Sprintf("Associate only supports a single key, found %d", len(h.childHelper.Keys)))
		}

		f.Func().Params(receiverID().Op("*").Id(implName)).Id(
			fmt.Sprintf("Associate%s", pl.Plural(h.childHelper.StructName))).Params(ctxID().Add(data.QualCtx), keyID().Add(kc), addID().Index().Add(ckc), removeID().Index().Add(ckc)).
			Params(jen.Error()).
			Block(jen.Return(
				jen.Qual(ImportThis, "Associate").Types(kc, ckc).Call(ctxID(), jen.Qual(ImportThis, "AssociateParams").Types(kc, ckc).Block(
					jen.Id("AssociationTable").Op(":").Lit(fmt.Sprintf("%s_%s", h.parentHelper.TablePrefix, h.childHelper.TableName)).Op(","),
					jen.Id("ParentColumnName").Op(":").Lit(fmt.Sprintf("%s_%s", h.parentHelper.TablePrefix,
						h.parentHelper.Keys[0].Name)).Op(","),
					jen.Id("ChildColumnName").Op(":").Lit(fmt.Sprintf("%s_%s", h.childHelper.TablePrefix,
						h.childHelper.Keys[0].Name)).Op(","),
					jen.Id("ParentKey").Op(":").Add(keyID()).Op(","),
					jen.Id("Add").Op(":").Add(addID()).Op(","),
					jen.Id("Remove").Op(":").Add(removeID()).Op(","),
					jen.Id("ParentRepository").Op(":").Add(receiverID()).Op(","),
					jen.Id("ChildRepository").Op(":").Add(receiverID()).Dot(fmt.Sprintf("%sRepository", strcase.ToLowerCamel(h.childHelper.StructName))).Op(","),
				)),
			))
	})
}

// addFilterKeysHandlers registers statement handlers to process implementations of FilterKeys in the provided HandlerEntries.
// It ensures compatibility with ImplFields and ImplFieldAssignments types and handles filters by generating appropriate templates.
func addFilterKeysHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.FilterKeys](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFields)
		d := _if.Entry
		jh := d.GetJenHelper()

		return s.Add(jen.Qual(data.ImportThis, "FilterKeysTemplate").Types(jh.GenerateKeyCode(_if.InterfaceImport)))

	}).AddStatementHandler(genlib.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.FilterKeys](in.Entry)
	}), func(s *jen.Statement, _ genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := GetGormJenHelper(d)

		internalName := jh.StructName + "Internal"

		typs := &jen.Statement{}
		typs.Add(jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName), jh.GenerateKeyCode(_if.InterfaceImport))

		if len(jh.Keys) != 1 {
			fmt.Println(jh.Keys)
			panic(fmt.Sprintf("FilterKeys only supports a single key, found %d", len(jh.Keys)))
		}

		return s.Add(jen.Id("FilterKeysTemplate").Op(":").Qual(ImportThis, "NewMappingFilterKeysTemplate").Types(*typs...).
			Params(jen.Qual(ImportThis, "MappingFilterKeysTemplateImplOptions").Types(*typs...).
				Block(
					jen.Id("Template").Op(":").Id("template").Op(","),
					jen.Id("FindColumn").Op(":").Lit(jh.Keys[0].Name).Op(","),
				)).Op(","))

	})

}

// addListByAssociatedKeyHandlers adds file handlers for ListByAssociatedKey functionality in the provided HandlerEntries.
// It generates methods to list items by associated keys, ensuring constraints like the presence of a single key.
func addListByAssociatedKeyHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddFileHandler(genlib.NewKeyWithTest[*FileMain](func(in *FileMain) bool {
		return data.HasImplementation[data.ListByAssociatedKey](in.Entry)
	}), func(f *jen.File, _ genlib.Registry, entry any) {

		i := entry.(*FileMain)

		a := data.GetImplementation[data.ListByAssociatedKey](i.Entry)

		jh := GetGormJenHelper(i.Entry)

		_e := &data.Entry{
			Type: a.AssociatedType,
		}

		jha := GetGormJenHelper(_e)

		cka := jha.GenerateKeyCode("")

		receiverID := func() *jen.Statement { return jen.Id("r") }

		implName := strcase.ToLowerCamel(jh.StructName) + "RepositoryImpl"

		if len(jh.Keys) != 1 || len(jha.Keys) != 1 {
			panic(fmt.Sprintf("ListByAssociatedKey only supports a single key, found %d and %d", len(jh.Keys), len(jha.Keys)))
		}

		ctxName := "ctx"
		keyName := "key"
		paramsName := "params"
		txName := "tx"
		thisTable := jh.TableName
		var assocatedTable string
		if !a.Reversed {
			assocatedTable = fmt.Sprintf("%s_%s", jh.TablePrefix, jha.TableName)
		} else {
			assocatedTable = fmt.Sprintf("%s_%s", jha.TablePrefix, jh.TableName)
		}
		thisAssocatedKey := fmt.Sprintf("%s_%s", jh.TablePrefix, jh.Keys[0].Name)
		otherAssocatedKey := fmt.Sprintf("%s_%s", jha.TablePrefix, jha.Keys[0].Name)

		f.Func().Params(receiverID().Op("*").Id(implName)).Id(fmt.Sprintf("ListBy%s", jha.StructName)).Params(
			jen.Id(ctxName).Add(data.QualCtx),
			jen.Id(keyName).Add(cka),
			jen.Id(paramsName).Qual(data.ImportThis, "ListParams"),
		).Params(
			jen.Op("*").Qual(data.ImportThis, "List").Types(
				jen.Op("*").Add(jh.StructType),
			),
			jen.Error(),
		).Block(
			jen.Return(receiverID().Dot("Template").Dot("DoList").Call(
				jen.Id(ctxName),
				jen.Func().Params(
					jen.Id(txName).Op("*").Qual(ImportGorm, "DB")).Params(jen.Op("*").Qual(ImportGorm, "DB")).Block(
					jen.Return(jen.Id(txName).Dot("Joins").Call(jen.Lit(
						fmt.Sprintf("INNER JOIN %s ON %s.%s = %s.%s",
							assocatedTable,
							assocatedTable,
							thisAssocatedKey,
							thisTable,
							jh.Keys[0].Name,
						),
					)).
						Dot("Where").Call(jen.Lit(
						fmt.Sprintf("%s.%s=?",
							assocatedTable, otherAssocatedKey,
						),
					),
						jen.Id(keyName),
					)),
				),
				jen.Id(paramsName),
			),
			),
		)
	})
}

// NewDataRegistry initializes a new genlib.Registry with predefined sets of handler entries for various operations.
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
