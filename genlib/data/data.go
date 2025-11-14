package data

import (
	"github.com/activatedio/datainfra/genlib"
	"github.com/dave/jennifer/jen"
)

var (
	ImportThis = "github.com/activatedio/datainfra/pkg/data"
)

type Types struct {
	Package string
	Entries []Entry
}

func (t *Types) GetPackage() string {
	return t.Package
}

type InterfaceMethods struct {
	Entry *Entry
}

func NewDataRegistry() genlib.Registry {

	return genlib.NewRegistry().WithHandlerEntries(genlib.NewHandlerEntries().AddFileHandler(&Types{}, func(f *jen.File, r genlib.Registry, entry any) {

		t := entry.(*Types)
		ds := t.Entries

		for _, d := range ds {

			impl := ExtractImplementationFor[Implementation](d.Implementations)

			if impl != nil {
				r = impl.RegistryBuilder(r.Clone())
			}

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

	}).AddStatementHandler(&InterfaceMethods{}, func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {

		i := entry.(*InterfaceMethods)
		d := i.Entry

		jh := d.GetJenHelper()

		for _, op := range d.Operations.All() {
			switch op {
			case OperationFindByKey:
				s.Add(jen.Id("FindByKey").Params(
					qualCtx,
					jh.GenerateKeyCode(""),
				).Params(
					jen.Op("*").Add(jh.StructType),
					idError,
				)).Add(jen.Id("ExistsByKey").Params(
					qualCtx,
					jh.GenerateKeyCode(""),
				).Params(
					jen.Bool(),
					idError,
				))
			case OperationList:
				s.Add(jen.Id("ListAll").Params(
					qualCtx, jen.Qual(ImportThis, "ListParams")).Params(
					jen.Op("*").Qual(ImportThis, "List").Types(
						jen.Op("*").Add(jh.StructType),
					),
					jen.Error(),
				))
			case OperationCreate:
				s.Add(jen.Id("Create").Params(
					qualCtx, jen.Op("*").Add(jh.StructType)).Params(
					jen.Error(),
				))
			case OperationUpdate:
				s.Add(jen.Id("Update").Params(
					qualCtx, jen.Op("*").Add(jh.StructType)).Params(
					jen.Error(),
				))
			case OperationDelete:
				s.Add(jen.Id("Delete").Params(
					qualCtx, jh.GenerateKeyCode("")).Params(
					jen.Error(),
				))
				s.Add(jen.Id("DeleteEntity").Params(
					qualCtx, jen.Op("*").Add(jh.StructType)).Params(
					jen.Error(),
				))
			}

			/*
				Create(ctx context.Context, entity E) error
				Update(ctx context.Context, entity E) error
				Delete(ctx context.Context, key K) error
				DeleteEntity(ctx context.Context, entity E) error

			*/
		}

		return s
	}))

}
