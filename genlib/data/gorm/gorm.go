package gorm

import (
	"fmt"
	"path/filepath"

	"github.com/activatedio/datainfra/genlib"
	"github.com/activatedio/datainfra/genlib/data"
	"github.com/dave/jennifer/jen"
	"github.com/iancoleman/strcase"
)

// ImportThis is a variable that defines the import path for the Gorm-related functionality within the repository.// ImportThis defines the string constant representing the import path for the GORM data package used in the application.
var (
	ImportThis = "github.com/activatedio/datainfra/pkg/data/gorm"
)

// DirectoryMain represents the main configuration structure for handling directory-based operations.
type DirectoryMain struct {
	Package         string
	InterfaceImport string
	GenerateIndex   bool
	IndexModule     string
	Entries         []data.Entry
}

// IndexMain represents the main index structure containing a module name and a list of data entries.
type IndexMain struct {
	IndexModule string
	Entries     []data.Entry
}

// FileMain represents the main file structure that includes an entry metadata and an interface import string.
type FileMain struct {
	Entry           *data.Entry
	InterfaceImport string
}

// InternalSuperFields is a structure that wraps a pointer to a data.Entry instance, representing a data descriptor.
type InternalSuperFields struct {
	Entry *data.Entry
}

// InternalFields represents a structure intended for internal data handling within the application.
type InternalFields struct{}

// InternalFunctions provides utility methods to handle the internal operations within the framework.// InternalFunctions is a struct used as a key or marker for handling specific internal functionalities in a registry.
type InternalFunctions struct{}

// ImplFields represents implementation details and associated metadata for a specific interface or entry.
// Entry refers to the data type descriptor, while InterfaceImport is the relevant interface's import path.
type ImplFields struct {
	Entry           *data.Entry
	InterfaceImport string
}

// ImplFieldAssignments is a structure that binds data entries with their corresponding interface imports.
// Entry represents the specific data entry tied to the implementation.
// InterfaceImport specifies the import path for the corresponding interface.
type ImplFieldAssignments struct {
	Entry           *data.Entry
	InterfaceImport string
}

// CtorParamsFields is a placeholder type used for constructing parameters in dependency injection scenarios.
type CtorParamsFields struct {
	Entry           *data.Entry
	InterfaceImport string
}

// Ctor represents a constructor structure containing a reference to a data.Entry instance.
type Ctor struct {
	Entry *data.Entry
}

type TemplateFields struct{}

// TemplateParamsField represents a struct used for defining parameters or fields within a template.
type TemplateParamsField struct{}

// CrudTemplateParamsField defines the structure for parameters used in CRUD templates.
type CrudTemplateParamsField struct{}

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

	}).AddFileHandler(genlib.NewKey[*IndexMain](), func(f *jen.File, r genlib.Registry, entry any) {

		im := entry.(*IndexMain)

		opts := &jen.Statement{}

		opts.Add(
			jen.Qual(ImportThis, "NewDB"),
			jen.Qual(ImportThis, "NewContextBuilder"),
		)

		for _, d := range im.Entries {
			opts = opts.Add(jen.Id(fmt.Sprintf("New%sRepository", d.Type.Name())))
		}

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
		fs = append(fs, *r.BuildStatement(&jen.Statement{}, &InternalFields{})...)
		f.Commentf("%s is the internal representation of %s", internalName, jh.StructName)
		f.Type().Id(internalName).Struct(fs...)
		r.RunFileHandler(f, &InternalFunctions{})

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
	}).AddStatementHandler(genlib.NewKey[*InternalSuperFields](), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		fm := entry.(*InternalSuperFields)
		d := fm.Entry
		jh := d.GetJenHelper()
		return s.Add(jen.Op("*").Add(jh.StructType))

	}).AddStatementHandler(genlib.NewKey[*Ctor](), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		fm := entry.(*Ctor)
		d := fm.Entry
		jh := GetGormJenHelper(d)

		internalName := jh.StructName + "Internal"
		tmplStmt := &jen.Statement{}
		r.BuildStatement(tmplStmt, &TemplateFields{})
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
	}).AddStatementHandler(genlib.NewKey[*ImplFieldAssignments](), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {
		return s.Add(jen.Id("Template").Op(":").Id("template").Op(","))
	})
}

func addCrudHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.Crud](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

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

func addSearchHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.Search](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFields)
		d := _if.Entry
		jh := d.GetJenHelper()

		return s.Add(jen.Qual(data.ImportThis, "SearchTemplate").Types(
			jen.Op("*").Add(jh.StructType),
		))

	}).AddStatementHandler(genlib.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.Search](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := d.GetJenHelper()

		internalName := jh.StructName + "Internal"
		return s.Add(jen.Id("SearchTemplate").Op(":").Qual(ImportThis, "NewMappingSearchTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Params(jen.Qual(ImportThis, "MappingSearchTemplateParams").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Block(
			jen.Id("Template").Op(":").Id("template").Op(","),
		)).Op(","))

	})

}

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
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		f := entry.(*ImplFields)
		h := toHelper(f.Entry)

		return s.Add(jen.Id(fmt.Sprintf("%sRepository", strcase.ToLowerCamel(h.childHelper.StructName))).Qual(f.InterfaceImport, h.childHelper.InterfaceName))

	}).AddStatementHandler(genlib.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		f := entry.(*ImplFieldAssignments)
		h := toHelper(f.Entry)

		return s.Add(jen.Id(fmt.Sprintf("%sRepository", strcase.ToLowerCamel(h.childHelper.StructName))).Op(":").
			Id("params").
			Dot(fmt.Sprintf("%sRepository", h.childHelper.StructName)).Op(","))

	}).AddStatementHandler(genlib.NewKeyWithTest[*CtorParamsFields](func(in *CtorParamsFields) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		f := entry.(*CtorParamsFields)
		h := toHelper(f.Entry)

		return s.Add(jen.Id(fmt.Sprintf("%sRepository", h.childHelper.StructName)).
			Qual(f.InterfaceImport, fmt.Sprintf("%sRepository", h.childHelper.StructName)))

	}).AddFileHandler(genlib.NewKeyWithTest[*FileMain](func(in *FileMain) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(f *jen.File, r genlib.Registry, entry any) {

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

		f.Func().Params(receiverID().Op("*").Id(implName)).Id(
			fmt.Sprintf("Associate%s", pl.Plural(h.childHelper.StructName))).Params(ctxID().Add(data.QualCtx), keyID().Add(kc), addID().Index().Add(ckc), removeID().Index().Add(ckc)).
			Params(jen.Error()).
			Block(jen.Return(
				jen.Qual(ImportThis, "Associate").Types(kc, ckc).Call(ctxID(), jen.Qual(ImportThis, "AssociateParams").Types(kc, ckc).Block(
					jen.Id("AssociationTable").Op(":").Lit(fmt.Sprintf("%s_%s", h.parentHelper.TablePrefix, h.childHelper.TableName)).Op(","),
					jen.Id("ParentColumnName").Op(":").Lit(fmt.Sprintf("%s_%s", h.parentHelper.TablePrefix, "id")).Op(","),
					jen.Id("ChildColumnName").Op(":").Lit(fmt.Sprintf("%s_%s", h.childHelper.TablePrefix, "id")).Op(","),
					jen.Id("ParentKey").Op(":").Add(keyID()).Op(","),
					jen.Id("Add").Op(":").Add(addID()).Op(","),
					jen.Id("Remove").Op(":").Add(removeID()).Op(","),
					jen.Id("ParentRepository").Op(":").Add(receiverID()).Op(","),
					jen.Id("ChildRepository").Op(":").Add(receiverID()).Dot(fmt.Sprintf("%sRepository", strcase.ToLowerCamel(h.childHelper.StructName))).Op(","),
				)),
			))
	})
}

func addFilterKeysHandlers(he *genlib.HandlerEntries) *genlib.HandlerEntries {

	return he.AddStatementHandler(genlib.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.FilterKeys](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFields)
		d := _if.Entry
		jh := d.GetJenHelper()

		return s.Add(jen.Qual(data.ImportThis, "FilterKeysTemplate").Types(jh.GenerateKeyCode(_if.InterfaceImport)))

	}).AddStatementHandler(genlib.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.FilterKeys](in.Entry)
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := GetGormJenHelper(d)

		internalName := jh.StructName + "Internal"

		typs := &jen.Statement{}
		typs.Add(jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName), jh.GenerateKeyCode(_if.InterfaceImport))

		if len(jh.Keys) != 1 {
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

// NewDataRegistry creates a new data registry configured with custom handler entries for directories, files, and statements.
func NewDataRegistry() genlib.Registry {

	he := genlib.NewHandlerEntries()

	he = addBaseHandlers(he)
	he = addCrudHandlers(he)
	he = addSearchHandlers(he)
	he = addAssociateHandlers(he)
	he = addFilterKeysHandlers(he)

	return genlib.NewRegistry().WithHandlerEntries(he)
}
