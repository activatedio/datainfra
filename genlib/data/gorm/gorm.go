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
	Entries         []data.Entry
}

type FileMain struct {
	Entry           *data.Entry
	InterfaceImport string
}

func (m *DirectoryMain) GetPackage() string {
	return m.Package
}

type InternalSuperFields struct{}
type InternalFields struct{}
type InternalFunctions struct{}
type ImplFields struct{}
type ImplFieldAssignments struct{}
type CtorParamsFields struct{}
type Ctor struct{}
type TemplateFields struct{}
type TemplateParamsField struct{}

func NewDataRegistry() genlib.Registry {

	return genlib.NewRegistry().WithHandlerEntries(genlib.
		NewHandlerEntries().AddDirectoryHandler(&DirectoryMain{}, func(dirPath string, r genlib.Registry, entry any) {

		m := entry.(*DirectoryMain)

		for _, e := range m.Entries {
			genlib.WithFile(m.GetPackage(), filepath.Join(dirPath, fmt.Sprintf("%s_gen.go", strcase.ToSnake(e.Type.Name()))), func(file *jen.File) {
				r.RunFileHandler(file, &FileMain{
					InterfaceImport: m.InterfaceImport,
					Entry:           &e,
				})
			})
		}

	}).AddFileHandler(&FileMain{}, func(f *jen.File, r genlib.Registry, entry any) {

		fm := entry.(*FileMain)
		d := fm.Entry
		jh := d.GetJenHelper()
		internalName := jh.StructName + "Internal"
		implName := strcase.ToLowerCamel(jh.StructName) + "RepositoryImpl"

		fs := *r.BuildStatement(&jen.Statement{}, &InternalSuperFields{})
		fs = append(fs, *r.BuildStatement(&jen.Statement{}, &InternalFields{})...)
		f.Commentf("%s is the internal representation of %s", internalName, jh.StructName)
		f.Type().Id(internalName).Struct(fs...)
		r.RunFileHandler(f, &InternalFunctions{})

		implFields := &jen.Statement{}
		implFields.Add(jen.Id("Template").Qual(ImportThis, "MappingTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName)))
		implFields = r.BuildStatement(implFields, &ImplFields{})
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
		r.BuildStatement(ctor, &Ctor{})

		ctor.Add(jen.Return(jen.Op("&").Qual("", implName).Block(
			r.BuildStatement(&jen.Statement{}, &ImplFieldAssignments{}),
		)))

		paramsID := "params"

		if len(*cpfStmt) == 1 {
			paramsID = "_"
		}

		f.Commentf("New%sRepository creates a new %sRepository", jh.StructName, jh.StructName)
		f.Func().Id(fmt.Sprintf("New%sRepository", jh.StructName)).Params(
			jen.Id(paramsID).Id(paramsType),
		).Qual(fm.InterfaceImport, jh.InterfaceName).Block(*ctor...).Line()
	}))

}
