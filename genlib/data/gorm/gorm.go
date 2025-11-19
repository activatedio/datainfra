package gorm

import (
	"fmt"
	"path/filepath"

	"github.com/activatedio/datainfra/genlib"
	"github.com/activatedio/datainfra/genlib/data"
	"github.com/dave/jennifer/jen"
	"github.com/iancoleman/strcase"
)

var (
	ImportThis = "github.com/activatedio/datainfra/pkg/data/gorm"
)

type DirectoryMain struct {
	Package         string
	InterfaceImport string
	GenerateIndex   bool
	IndexModule     string
	Entries         []data.Entry
}

type IndexMain struct {
	IndexModule string
	Entries     []data.Entry
}

type FileMain struct {
	Entry           *data.Entry
	InterfaceImport string
}

type InternalSuperFields struct {
	Entry *data.Entry
}
type InternalFields struct{}
type InternalFunctions struct{}
type ImplFields struct {
	Entry           *data.Entry
	InterfaceImport string
}
type ImplFieldAssignments struct {
	Entry           *data.Entry
	InterfaceImport string
}
type CtorParamsFields struct{}
type Ctor struct {
	Entry *data.Entry
}
type TemplateFields struct{}
type TemplateParamsField struct{}

type CrudTemplateParamsField struct{}

func NewDataRegistry() genlib.Registry {

	return genlib.NewRegistry().WithHandlerEntries(genlib.
		NewHandlerEntries().AddDirectoryHandler(genlib.NewKey[*DirectoryMain](), func(dirPath string, r genlib.Registry, entry any) {

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
		cpfStmt.Add(*r.BuildStatement(&jen.Statement{}, &CtorParamsFields{})...)

		f.Commentf("%s are the parameters for %sRepository", paramsType, jh.StructName)
		f.Type().Id(paramsType).Struct(*cpfStmt...)

		ctor := &jen.Statement{}
		r.BuildStatement(ctor, &Ctor{
			Entry: d,
		})

		ctor.Add(jen.Return(jen.Op("&").Qual("", implName).Block(
			r.BuildStatement(&jen.Statement{}, &ImplFieldAssignments{
				Entry:           d,
				InterfaceImport: fm.InterfaceImport,
			}),
		)))

		paramsID := "params"

		if len(*cpfStmt) == 1 {
			paramsID = "_"
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
	}).AddStatementHandler(genlib.NewKey[*ImplFields](), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFields)
		d := _if.Entry
		jh := d.GetJenHelper()
		// Determine if we have any crud operations
		if d.Operations.Intersect(data.OperationsCrud).Len() == 0 {
			// Short circuit
			return s
		}

		return s.Add(jen.Qual(data.ImportThis, "CrudTemplate").Types(
			jen.Op("*").Add(jh.StructType),
			jh.GenerateKeyCode(_if.InterfaceImport),
		))

	}).AddStatementHandler(genlib.NewKey[*ImplFieldAssignments](), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := GetGormJenHelper(d)
		if d.Operations.Intersect(data.OperationsCrud).Len() == 0 {
			// Short circuit
			return s
		}

		internalName := jh.StructName + "Internal"

		crudParamsFields := r.BuildStatement(&jen.Statement{}, &CrudTemplateParamsField{})

		if len(jh.KeyFields) == 1 {
			crudParamsFields.Add(jen.Id("FindBuilder").Op(":").Qual(ImportThis, "SingleFindBuilder").Types(
				jh.GenerateKeyCode(_if.InterfaceImport)).Params(jen.Lit(fmt.Sprintf("%s.%s", jh.TableName, jh.KeyFields[0].Name))).Op(","))
		}

		return s.Id("CrudTemplate").Op(":").Qual(ImportThis, "NewMappingCrudTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName), jh.GenerateKeyCode(_if.InterfaceImport),
		).Params(jen.Qual(ImportThis, "MappingCrudTemplateImplOptions").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName), jh.GenerateKeyCode(_if.InterfaceImport),
		).Block(
			jen.Id("Template").Op(":").Id("template").Op(","),
			crudParamsFields,
		)).Op(",")

	}).AddStatementHandler(genlib.NewKeyWithTest[*Ctor](func(in *Ctor) bool {
		_, ok := data.GetImplementation[data.Search](in.Entry)
		return ok
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {
		return s.Add(jen.Commentf("implements the SearchHandler interface."))
	}).AddStatementHandler(genlib.NewKeyWithTest[*Ctor](func(in *Ctor) bool {
		_, ok := data.GetImplementation[data.Associate](in.Entry)
		return ok
	}), func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {
		return s.Add(jen.Commentf("implements the SearchHandler interface."))
	}))

}
