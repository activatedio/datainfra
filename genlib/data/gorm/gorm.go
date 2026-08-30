package gorm

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/activatedio/gen"
	"github.com/dave/jennifer/jen"
	"github.com/iancoleman/strcase"

	"github.com/activatedio/datainfra/genlib/data"
	pkgdata "github.com/activatedio/datainfra/pkg/data"
)

// ImportThis defines the import path for the gorm package utilized by the data infrastructure library.
var (
	ImportThis = "github.com/activatedio/datainfra/pkg/data/gorm"
)

// DirectoryMain represents a configuration for generating files and directories containing code based on supplied entries.
// Package defines the package name for generated files.
// InterfaceImport specifies the import path of the interfaces used by the entries.
// Wiring controls how generated constructors integrate with a DI framework
// (e.g. fx). Nil emits plain constructors with no framework imports and skips
// the index file. See data.Wiring.
// Entries is a collection of data Entry objects to process and use for code generation.
type DirectoryMain struct {
	Package         string
	InterfaceImport string
	Wiring          data.Wiring
	Entries         []data.Entry
}

// FileMain serves as a descriptor to facilitate code generation for a specific type using metadata from a data.Entry.
// Entry holds type-specific metadata and related operations for code generation.
// InterfaceImport specifies the import path for the target interface in the generated code.
// Wiring controls DI integration for the generated constructor; see DirectoryMain.
type FileMain struct {
	Entry           *data.Entry
	InterfaceImport string
	Wiring          data.Wiring
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

// Search contains the search predicate bindings for the gorm flavor. Each
// binding pairs the descriptor metadata exposed via GetSearchPredicates
// with a jen expression that constructs the runtime PredicateBinder. Use
// helpers such as PostgresKeywordsBinderCall and LikeBinderCall to produce
// well-formed Binder values.
type Search struct {
	Bindings []SearchBinding
}

// SearchBinding describes one entry in a gorm Search implementation. Name,
// Label, Virtual and Operators are surfaced through GetSearchPredicates.
// Binder must be jen code that evaluates to a gorm.PredicateBinder.
//
// Virtual marks the predicate as not corresponding to a real domain field
// (typically named with a leading "@" such as "@keywords"). See
// data.SearchPredicateDescriptor.Virtual for the wire semantics.
type SearchBinding struct {
	Name      string
	Label     string
	Virtual   bool
	Operators []pkgdata.SearchOperator
	Binder    jen.Code
}

// PostgresKeywordsBinderCall returns jen code that evaluates to
// gorm.PostgresKeywordsBinder(column).
func PostgresKeywordsBinderCall(column string) jen.Code {
	return jen.Qual(ImportThis, "PostgresKeywordsBinder").Call(jen.Lit(column))
}

// LikeBinderCall returns jen code that evaluates to gorm.LikeBinder(column).
func LikeBinderCall(column string) jen.Code {
	return jen.Qual(ImportThis, "LikeBinder").Call(jen.Lit(column))
}

// DialectBinderCall returns jen code that evaluates to gorm.DialectBinder
// over the supplied per-dialect map. Keys are emitted in sorted order so the
// generated source is deterministic.
func DialectBinderCall(byDialect map[string]jen.Code) jen.Code {
	keys := make([]string, 0, len(byDialect))
	for k := range byDialect {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]jen.Code, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, jen.Lit(k).Op(":").Add(byDialect[k]))
	}
	return jen.Qual(ImportThis, "DialectBinder").Call(
		jen.Map(jen.String()).Qual(ImportThis, "PredicateBinder").Values(pairs...),
	)
}

// TemplateFields defines a structured type used for mapping data between internal and external representations.
type TemplateFields struct{}

// CrudTemplateParamsField is a type used to define parameters for CRUD template configurations within a registry or handler.
type CrudTemplateParamsField struct{}

// Associate contains gorm-specific options
type Associate struct {
	ChildType     reflect.Type
	ExecuteRemove jen.Code
	ExecuteAdd    jen.Code
}

// addBaseHandlers configures and registers default directory, file, and statement handlers in the provided HandlerEntries.
func addBaseHandlers(he *gen.HandlerEntries) *gen.HandlerEntries {

	return he.AddDirectoryHandler(gen.NewKey[*DirectoryMain](), func(dirPath string, r gen.Registry, entry any) {

		m := entry.(*DirectoryMain)

		for _, e := range m.Entries {
			gen.WithFile(m.Package, filepath.Join(dirPath, fmt.Sprintf("%s_gen.go", strcase.ToSnake(e.Type.Name()))), func(file *jen.File) {
				r.RunFileHandler(file, &FileMain{
					InterfaceImport: m.InterfaceImport,
					Entry:           &e,
					Wiring:          m.Wiring,
				})
			})
		}

		if m.Wiring != nil {
			provideRefs := make([]jen.Code, 0, 2+len(m.Entries))
			provideRefs = append(provideRefs,
				jen.Qual(ImportThis, "NewDB"),
				jen.Qual(ImportThis, "NewContextBuilder"),
			)
			for _, e := range m.Entries {
				provideRefs = append(provideRefs, jen.Id(fmt.Sprintf("New%sRepository", e.Type.Name())))
			}
			gen.WithFile(m.Package, filepath.Join(dirPath, "index_gen.go"), func(file *jen.File) {
				m.Wiring.EmitIndex(file, provideRefs)
			})
		}

	}).AddFileHandler(gen.NewKey[*FileMain](), func(f *jen.File, r gen.Registry, entry any) {

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
		if fm.Wiring != nil {
			cpfStmt.Add(fm.Wiring.PrependCtorParamsFields()...)
		}
		prefixLen := len(*cpfStmt)
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

		if len(*cpfStmt) == prefixLen {
			paramsID = ""
		}

		f.Commentf("New%sRepository creates a new %sRepository", jh.StructName, jh.StructName)
		f.Func().Id(fmt.Sprintf("New%sRepository", jh.StructName)).Params(
			jen.Id(paramsID).Id(paramsType),
		).Qual(fm.InterfaceImport, jh.InterfaceName).Block(*ctor...).Line()
	}).AddStatementHandler(gen.NewKey[*InternalSuperFields](), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		fm := entry.(*InternalSuperFields)
		d := fm.Entry
		jh := d.GetJenHelper()
		return s.Add(jen.Op("*").Add(jh.StructType))

	}).AddStatementHandler(gen.NewKey[*Ctor](), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

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

		// Cursor pagination: emit KeyColumns + KeyAccessor for every entity
		// with at least one key. Composite keys land lexicographic
		// pagination via row-constructor WHERE clauses.
		if len(jh.Keys) >= 1 {
			cols := make([]jen.Code, len(jh.Keys))
			accessors := make([]jen.Code, len(jh.Keys))
			compositeRoot := jh.KeyField.Type.Kind() == reflect.Struct
			for i, k := range jh.Keys {
				cols[i] = jen.Lit(strcase.ToSnake(k.Name))
				if compositeRoot {
					accessors[i] = jen.Id("m").Dot(jh.KeyField.Name).Dot(k.Name)
				} else {
					accessors[i] = jen.Id("m").Dot(k.Name)
				}
			}
			tmplStmt.Add(jen.Id("KeyColumns").Op(":").Index().String().Values(cols...).Op(","))
			tmplStmt.Add(jen.Id("KeyAccessor").Op(":").Func().Params(
				jen.Id("m").Op("*").Id(internalName),
			).Index().Any().Block(
				jen.Return(jen.Index().Any().Values(accessors...)),
			).Op(","))
		}

		return s.Add(jen.Id("template").Op(":=").Qual(ImportThis, "NewMappingTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Call(jen.Qual(ImportThis, "MappingTemplateParams").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Block(*tmplStmt...)))
	}).AddStatementHandler(gen.NewKey[*ImplFieldAssignments](), func(s *jen.Statement, _ gen.Registry, _ any) *jen.Statement {
		return s.Add(jen.Id("Template").Op(":").Id("template").Op(","))
	})
}

// addCrudHandlers adds CRUD-specific statement handlers to the provided HandlerEntries based on certain entry conditions.
// It registers handlers for CRUD template generation and field assignments if CRUD operations are applicable.
func addCrudHandlers(he *gen.HandlerEntries) *gen.HandlerEntries {

	return he.AddStatementHandler(gen.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.Crud](in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

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
			jh.GenerateKeyCode(),
		))

	}).AddStatementHandler(gen.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.Crud](in.Entry)
	}), func(s *jen.Statement, r gen.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := GetGormJenHelper(d)

		c := data.GetImplementation[data.Crud](d)
		i := data.GetImplementation[Implementation](d)

		if c.Operations.Intersect(data.OperationsCrud).Len() == 0 {
			// Short circuit
			return s
		}

		internalName := jh.StructName + "Internal"

		crudParamsFields := r.BuildStatement(&jen.Statement{}, &CrudTemplateParamsField{})

		switch {

		case i != nil && i.CustomFindBuilder != nil:
			crudParamsFields.Add(jen.Id("FindBuilder").Op(":").Add(i.CustomFindBuilder).Op(","))
		default:

			fields := &jen.Statement{}

			for _, f := range jh.Keys {
				fields.Add(jen.Qual(ImportThis, "FindPredicate").Types(jh.GenerateKeyCode()).Block(
					jen.Id("Accessor").Op(":").Func().Params(jen.Id(data.BaseKeyId).Add(jh.GenerateKeyCode())).Params(jen.Any()).Block(
						jen.Return().Add(f.Accessor),
					).Op(","),
					jen.Id("Column").Op(":").Lit(strcase.ToSnake(f.Name)).Op(","),
				))
			}

			crudParamsFields.Add(jen.Id("FindBuilder").Op(":").Qual(ImportThis, "NewFindBuilder").Types(
				jh.GenerateKeyCode()).Params(*fields...).Op(","))
		}

		return s.Add(jen.Id("CrudTemplate").Op(":").Qual(ImportThis, "NewMappingCrudTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName), jh.GenerateKeyCode(),
		).Params(jen.Qual(ImportThis, "MappingCrudTemplateImplOptions").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName), jh.GenerateKeyCode(),
		).Block(
			jen.Id("Template").Op(":").Id("template").Op(","),
			crudParamsFields,
		)).Op(","))

	})

}

// addSearchHandlers registers statement handlers for search implementations in handler entries and returns the updated instance.
func addSearchHandlers(he *gen.HandlerEntries) *gen.HandlerEntries {

	return he.AddStatementHandler(gen.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		// Need both top level and gorm search
		return data.HasImplementation[data.Search](in.Entry) && data.HasImplementation[Search](in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFields)
		d := _if.Entry
		jh := d.GetJenHelper()

		return s.Add(jen.Qual(data.ImportThis, "SearchTemplate").Types(
			jen.Op("*").Add(jh.StructType),
		))

	}).AddStatementHandler(gen.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.Search](in.Entry) && data.HasImplementation[Search](in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := d.GetJenHelper()

		gs := data.GetImplementation[Search](d)

		var bindings jen.Code
		if len(gs.Bindings) == 0 {
			bindings = jen.Nil()
		} else {
			bindings = generateSearchBindings(gs.Bindings)
		}

		internalName := jh.StructName + "Internal"
		return s.Add(jen.Id("SearchTemplate").Op(":").Qual(ImportThis, "NewMappingSearchTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Params(jen.Qual(ImportThis, "MappingSearchTemplateParams").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Block(
			jen.Id("Template").Op(":").Id("template").Op(","),
			jen.Id("Bindings").Op(":").Add(bindings).Op(","),
		)).Op(","))

	})

}

func generateSearchBindings(bindings []SearchBinding) *jen.Statement {

	entries := &jen.Statement{}
	for _, b := range bindings {
		if b.Binder == nil {
			panic(fmt.Sprintf("gorm.SearchBinding %q is missing a Binder; use PostgresKeywordsBinderCall or LikeBinderCall", b.Name))
		}

		ops := make([]jen.Code, 0, len(b.Operators))
		for _, op := range b.Operators {
			ops = append(ops, operatorJenCode(op))
		}

		descriptorFields := &jen.Statement{
			jen.Id("Name").Op(":").Lit(b.Name).Op(","),
			jen.Id("Label").Op(":").Lit(b.Label).Op(","),
		}
		if b.Virtual {
			descriptorFields.Add(jen.Id("Virtual").Op(":").Lit(true).Op(","))
		}
		descriptorFields.Add(jen.Id("Operators").Op(":").Index().Qual(data.ImportThis, "SearchOperator").Block(ops...).Op(","))

		entries.Add(jen.Block(
			jen.Id("Descriptor").Op(":").Op("&").Qual(data.ImportThis, "SearchPredicateDescriptor").Block(*descriptorFields...).Op(","),
			jen.Id("Binder").Op(":").Add(b.Binder).Op(","),
		).Op(","))
	}

	return jen.Index().Qual(ImportThis, "SearchPredicateBinding").Block(*entries...)
}

func operatorJenCode(op pkgdata.SearchOperator) *jen.Statement {
	names := map[pkgdata.SearchOperator]string{
		pkgdata.SearchOperatorNumberEquals:    "SearchOperatorNumberEquals",
		pkgdata.SearchOperatorStringEquals:    "SearchOperatorStringEquals",
		pkgdata.SearchOperatorStringNotEquals: "SearchOperatorStringNotEquals",
		pkgdata.SearchOperatorStringMatch:     "SearchOperatorStringMatch",
		pkgdata.SearchOperatorStringIn:        "SearchOperatorStringIn",
		pkgdata.SearchOperatorStringNotIn:     "SearchOperatorStringNotIn",
		pkgdata.SearchOperatorNumberNotEquals: "SearchOperatorNumberNotEquals",
		pkgdata.SearchOperatorNumberIn:        "SearchOperatorNumberIn",
	}
	if name, ok := names[op]; ok {
		return jen.Qual(data.ImportThis, name).Op(",")
	}
	panic(fmt.Sprintf("unknown SearchOperator %d", op))
}

// addAssociateHandlers adds handlers to facilitate the management of associate relationships between data entities.
// It updates the given HandlerEntries by registering statement and file handlers for entries with 'Associate' implementations.
// Returns the updated HandlerEntries instance.
func addAssociateHandlers(he *gen.HandlerEntries) *gen.HandlerEntries { //nolint:gocyclo // higher complexity is okay for this

	type helper struct {
		targetType   reflect.Type
		parentHelper JenHelper
		childHelper  JenHelper
	}

	toHelper := func(e *data.Entry) []helper {

		associates := data.GetImplementations[data.Associate](e)
		res := make([]helper, 0, len(associates))

		for _, a := range associates {

			_e := &data.Entry{
				Type: a.ChildType,
			}

			res = append(res, helper{
				targetType:   e.Type,
				parentHelper: GetGormJenHelper(e),
				childHelper:  GetGormJenHelper(_e),
			})
		}

		return res
	}

	return he.AddStatementHandler(gen.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		f := entry.(*ImplFields)

		for _, h := range toHelper(f.Entry) {
			s.Add(jen.Id(fmt.Sprintf("%sRepository", strcase.ToLowerCamel(h.childHelper.StructName))).Qual(f.InterfaceImport, h.childHelper.InterfaceName))
		}

		return s

	}).AddStatementHandler(gen.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		f := entry.(*ImplFieldAssignments)
		for _, h := range toHelper(f.Entry) {
			s.Add(jen.Id(fmt.Sprintf("%sRepository", strcase.ToLowerCamel(h.childHelper.StructName))).Op(":").
				Id("params").
				Dot(fmt.Sprintf("%sRepository", h.childHelper.StructName)).Op(","))
		}

		return s

	}).AddStatementHandler(gen.NewKeyWithTest[*CtorParamsFields](func(in *CtorParamsFields) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		f := entry.(*CtorParamsFields)
		for _, h := range toHelper(f.Entry) {
			s.Add(jen.Id(fmt.Sprintf("%sRepository", h.childHelper.StructName)).
				Qual(f.InterfaceImport, fmt.Sprintf("%sRepository", h.childHelper.StructName)))
		}

		return s

	}).AddFileHandler(gen.NewKeyWithTest[*FileMain](func(in *FileMain) bool {
		return data.HasImplementation[data.Associate](in.Entry)
	}), func(f *jen.File, _ gen.Registry, entry any) {

		fm := entry.(*FileMain)
		for _, h := range toHelper(fm.Entry) {

			kc := h.parentHelper.GenerateKeyCode()
			ckc := h.childHelper.GenerateKeyCode()

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

			pFieldsStmt := &jen.Statement{
				jen.Id("AssociationTable").Op(":").Lit(fmt.Sprintf("%s_%s", h.parentHelper.TablePrefix, h.childHelper.TableName)).Op(","),
				jen.Id("ParentColumnName").Op(":").Lit(fmt.Sprintf("%s_%s", h.parentHelper.TablePrefix,
					strcase.ToSnake(h.parentHelper.Keys[0].Name))).Op(","),
				jen.Id("ChildColumnName").Op(":").Lit(fmt.Sprintf("%s_%s", h.childHelper.TablePrefix,
					strcase.ToSnake(h.childHelper.Keys[0].Name))).Op(","),
				jen.Id("ParentKey").Op(":").Add(keyID()).Op(","),
				jen.Id("Add").Op(":").Add(addID()).Op(","),
				jen.Id("Remove").Op(":").Add(removeID()).Op(","),
				jen.Id("ParentRepository").Op(":").Add(receiverID()).Op(","),
				jen.Id("ChildRepository").Op(":").Add(receiverID()).Dot(fmt.Sprintf("%sRepository", strcase.ToLowerCamel(h.childHelper.StructName))).Op(","),
			}

			ai := data.GetImplementation[Associate](fm.Entry, data.WithTest[Associate](func(in Associate) {
				in.ChildType = h.targetType
			}))

			if ai != nil {
				if ai.ExecuteAdd != nil {
					pFieldsStmt.Add(jen.Id("ExecuteAdd").Op(":").Add(ai.ExecuteAdd).Op(","))
				}
				if ai.ExecuteRemove != nil {
					pFieldsStmt.Add(jen.Id("ExecuteRemove").Op(":").Add(ai.ExecuteRemove).Op(","))
				}
			}

			f.Func().Params(receiverID().Op("*").Id(implName)).Id(
				fmt.Sprintf("Associate%s", pl.Plural(h.childHelper.StructName))).Params(ctxID().Add(data.QualCtx), keyID().Add(kc), addID().Index().Add(ckc), removeID().Index().Add(ckc)).
				Params(jen.Error()).
				Block(jen.Return(
					jen.Qual(ImportThis, "Associate").Types(kc, ckc).Call(ctxID(), jen.Qual(ImportThis, "AssociateParams").Types(kc, ckc).Block(
						*pFieldsStmt...,
					)),
				))
		}
	})
}

// addFilterKeysHandlers registers statement handlers to process implementations of FilterKeys in the provided HandlerEntries.
// It ensures compatibility with ImplFields and ImplFieldAssignments types and handles filters by generating appropriate templates.
func addFilterKeysHandlers(he *gen.HandlerEntries) *gen.HandlerEntries {

	return he.AddStatementHandler(gen.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.FilterKeys](in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFields)
		d := _if.Entry
		jh := d.GetJenHelper()

		return s.Add(jen.Qual(data.ImportThis, "FilterKeysTemplate").Types(jh.GenerateKeyCode()))

	}).AddStatementHandler(gen.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.FilterKeys](in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := GetGormJenHelper(d)

		internalName := jh.StructName + "Internal"

		typs := &jen.Statement{}
		typs.Add(jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName), jh.GenerateKeyCode())

		if len(jh.Keys) != 1 {
			fmt.Println(jh.Keys)
			panic(fmt.Sprintf("FilterKeys only supports a single key, found %d", len(jh.Keys)))
		}

		return s.Add(jen.Id("FilterKeysTemplate").Op(":").Qual(ImportThis, "NewMappingFilterKeysTemplate").Types(*typs...).
			Params(jen.Qual(ImportThis, "MappingFilterKeysTemplateImplOptions").Types(*typs...).
				Block(
					jen.Id("Template").Op(":").Id("template").Op(","),
					jen.Id("FindColumn").Op(":").Lit(strcase.ToSnake(jh.Keys[0].Name)).Op(","),
				)).Op(","))

	})

}

// addListByAssociatedKeyHandlers adds file handlers for ListByAssociatedKey functionality in the provided HandlerEntries.
// It generates methods to list items by associated keys, ensuring constraints like the presence of a single key.
func addListByAssociatedKeyHandlers(he *gen.HandlerEntries) *gen.HandlerEntries {

	return he.AddFileHandler(gen.NewKeyWithTest[*FileMain](func(in *FileMain) bool {
		return data.HasImplementation[data.ListByAssociatedKey](in.Entry)
	}), func(f *jen.File, _ gen.Registry, entry any) {

		i := entry.(*FileMain)

		for _, a := range data.GetImplementations[data.ListByAssociatedKey](i.Entry) {

			jh := GetGormJenHelper(i.Entry)

			_e := &data.Entry{
				Type: a.AssociatedType,
			}

			jha := GetGormJenHelper(_e)

			cka := jha.GenerateKeyCode()

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
			thisAssocatedKey := fmt.Sprintf("%s_%s", jh.TablePrefix, strcase.ToSnake(jh.Keys[0].Name))
			otherAssocatedKey := fmt.Sprintf("%s_%s", jha.TablePrefix, strcase.ToSnake(jha.Keys[0].Name))

			// When the edge table carries the scope column, constrain it too:
			// a name-keyed edge is otherwise matched across every scope. Only
			// declared edges get this — see ListByAssociatedKey.ScopedEdge for
			// why it cannot be inferred.
			edgeScope := jen.Return(jen.Id(txName))
			if a.ScopedEdge {
				edgeScope = jen.Return(receiverID().Dot("Template").
					Dot("ApplyContextScopeQueryBuilderForTable").Call(
					jen.Id(ctxName),
					jen.Id(txName),
					jen.Lit(assocatedTable),
					jen.Qual(data.ImportThis, "FetchTypeList"),
				))
			}

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
						// tx = tx.Joins(...).Where(<edge>.<otherKey>=?, key)
						jen.Id(txName).Op("=").Id(txName).Dot("Joins").Call(jen.Lit(
							fmt.Sprintf("INNER JOIN %s ON %s.%s = %s.%s",
								assocatedTable,
								assocatedTable,
								thisAssocatedKey,
								thisTable,
								strcase.ToSnake(jh.Keys[0].Name),
							),
						)).
							Dot("Where").Call(jen.Lit(
							fmt.Sprintf("%s.%s=?",
								assocatedTable, otherAssocatedKey,
							),
						),
							jen.Id(keyName),
						),
						edgeScope,
					),
					jen.Id(paramsName),
				),
				),
			)
		}
	})
}

// hasGormHistogram reports whether the entry has a non-nil gorm.Histogram
// configured on its gorm.Implementation. The stub handler keys off the
// inverse so entries that opt into data.SearchHistogram without also
// configuring gorm.Histogram keep returning the historical error.
func hasGormHistogram(d *data.Entry) bool {
	if !data.HasImplementation[Implementation](d) {
		return false
	}
	return data.GetImplementation[Implementation](d).Histogram != nil
}

// addSearchHistogramStubHandlers emits an Unimplemented-stub
// SearchHistogram method on the gorm repository impl when the entry
// has the data.SearchHistogram marker but no gorm.Histogram
// configuration. Callers that want real Postgres-backed histograms set
// gorm.Histogram{DateColumn: ...} on the entry's gorm.Implementation;
// see addSearchHistogramHandlers.
func addSearchHistogramStubHandlers(he *gen.HandlerEntries) *gen.HandlerEntries {

	return he.AddFileHandler(gen.NewKeyWithTest[*FileMain](func(in *FileMain) bool {
		return data.HasImplementation[data.SearchHistogram](in.Entry) && !hasGormHistogram(in.Entry)
	}), func(f *jen.File, _ gen.Registry, entry any) {

		fm := entry.(*FileMain)
		jh := fm.Entry.GetJenHelper()

		implName := strcase.ToLowerCamel(jh.StructName) + "RepositoryImpl"

		f.Func().Params(jen.Id("r").Op("*").Id(implName)).Id("SearchHistogram").
			Params(
				jen.Id("_").Add(data.QualCtx),
				jen.Id("_").Op("[]*").Qual(data.ImportThis, "SearchPredicate"),
				jen.Id("_").Op("*").Qual(data.ImportThis, "HistogramSpec"),
			).
			Params(
				jen.Op("*").Qual(data.ImportThis, "HistogramResult"),
				jen.Error(),
			).
			Block(
				jen.Return(
					jen.Nil(),
					jen.Qual("fmt", "Errorf").Call(jen.Lit("SearchHistogram requires the Elasticsearch backend; gorm backend does not yet support date-histogram aggregations")),
				),
			)
	})

}

// addSearchHistogramHandlers wires a real Postgres-backed
// SearchHistogram method into the generated gorm repository when the
// entry sets a non-nil gorm.Histogram on its gorm.Implementation. The
// emitted impl struct embeds data.SearchHistogramTemplate[*Model] and
// the constructor wires it via gorm.NewMappingHistogramTemplate,
// reusing gorm.Search bindings so predicate filters match Search.
// SQLite (and any non-Postgres dialect) returns an error at runtime
// from the template.
func addSearchHistogramHandlers(he *gen.HandlerEntries) *gen.HandlerEntries {

	return he.AddStatementHandler(gen.NewKeyWithTest[*ImplFields](func(in *ImplFields) bool {
		return data.HasImplementation[data.SearchHistogram](in.Entry) && hasGormHistogram(in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFields)
		d := _if.Entry
		jh := d.GetJenHelper()

		return s.Add(jen.Qual(data.ImportThis, "SearchHistogramTemplate").Types(
			jen.Op("*").Add(jh.StructType),
		))

	}).AddStatementHandler(gen.NewKeyWithTest[*ImplFieldAssignments](func(in *ImplFieldAssignments) bool {
		return data.HasImplementation[data.SearchHistogram](in.Entry) && hasGormHistogram(in.Entry)
	}), func(s *jen.Statement, _ gen.Registry, entry any) *jen.Statement {

		_if := entry.(*ImplFieldAssignments)
		d := _if.Entry
		jh := d.GetJenHelper()

		hist := data.GetImplementation[Implementation](d).Histogram

		var bindings jen.Code
		if data.HasImplementation[Search](d) {
			gs := data.GetImplementation[Search](d)
			if len(gs.Bindings) == 0 {
				bindings = jen.Nil()
			} else {
				bindings = generateSearchBindings(gs.Bindings)
			}
		} else {
			bindings = jen.Nil()
		}

		internalName := jh.StructName + "Internal"
		return s.Add(jen.Id("SearchHistogramTemplate").Op(":").Qual(ImportThis, "NewMappingHistogramTemplate").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Params(jen.Qual(ImportThis, "MappingHistogramTemplateParams").Types(
			jen.Op("*").Add(jh.StructType), jen.Op("*").Qual("", internalName),
		).Block(
			jen.Id("Template").Op(":").Id("template").Op(","),
			jen.Id("Bindings").Op(":").Add(bindings).Op(","),
			jen.Id("DateColumn").Op(":").Lit(hist.DateColumn).Op(","),
		)).Op(","))

	})

}

// NewDataRegistry initializes a new genlib.Registry with predefined sets of handler entries for various operations.
func NewDataRegistry() gen.Registry {

	he := gen.NewHandlerEntries()

	he = addBaseHandlers(he)
	he = addCrudHandlers(he)
	he = addSearchHandlers(he)
	he = addSearchHistogramStubHandlers(he)
	he = addSearchHistogramHandlers(he)
	he = addAssociateHandlers(he)
	he = addFilterKeysHandlers(he)
	he = addListByAssociatedKeyHandlers(he)

	return gen.NewRegistry().WithHandlerEntries(he)
}
