package data

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/activatedio/datainfra/genlib"
	"github.com/dave/jennifer/jen"
)

type RegistryBuilder func(r genlib.Registry) genlib.Registry

// Entry is a descriptor for a data type
type Entry struct {
	Type       reflect.Type
	Operations *genlib.Set[Operation]
	// Implementations are implementation-specific parameters
	Implementations []any
}

func (d Entry) GetJenHelper() JenHelper {

	res := JenHelper{
		InterfaceName: fmt.Sprintf("%sRepository", d.Type.Name()),
		StructName:    d.Type.Name(),
	}

	res.StructType = jen.Qual(d.Type.PkgPath(), d.Type.Name())

	for i := 0; i < d.Type.NumField(); i++ {
		f := d.Type.Field(i)
		dt := ParseTag(f.Tag.Get("data"))
		if dt.IsKey {
			res.KeyFields = append(res.KeyFields, f)
		}
	}

	switch {
	case len(res.KeyFields) == 1:
		res.keyCodeGen = &fixedKeyCodeGenerator{
			code: jen.Qual(res.KeyFields[0].Type.PkgPath(), res.KeyFields[0].Type.Name()),
		}
	case len(res.KeyFields) > 1:
		keyTypeName := fmt.Sprintf("%sKey", d.Type.Name())
		res.keyCodeGen = &localKeyCodeGenerator{
			id: keyTypeName,
		}

		fs := &jen.Statement{}

		fs.Add(jen.Commentf("%s is the key for %s", keyTypeName, d.Type.Name()))

		for _, key := range res.KeyFields {
			fs.Add(jen.Id(key.Name).Qual(key.Type.PkgPath(), key.Type.Name()))
		}

		res.keyStmt = jen.Type().Id(keyTypeName).Struct(*fs...)
	}

	return res
}

type Tag struct {
	IsKey bool
}

func ParseTag(tag string) Tag {

	t := Tag{}

	for _, v := range strings.Split(tag, ",") {
		switch {
		case v == "key":
			t.IsKey = true
		}
	}

	return t
}

type keyCodeGenerator interface {
	// Generate generates a key code based on the relative import of the data interface type. For a local generation
	// this import will be blank.  The generator generates the code for the key, which may or may not use the
	// provided interface type depending on if the key type is local or not
	Generate(interfaceImport string) jen.Code
}

type fixedKeyCodeGenerator struct {
	code jen.Code
}

func (f *fixedKeyCodeGenerator) Generate(interfaceImport string) jen.Code {
	return f.code
}

type localKeyCodeGenerator struct {
	id string
}

func (l *localKeyCodeGenerator) Generate(interfaceImport string) jen.Code {
	if interfaceImport == "" {
		return jen.Id(l.id)
	} else {
		return jen.Qual(interfaceImport, l.id)
	}
}

type JenHelper struct {
	InterfaceName string
	StructType    jen.Code
	StructName    string
	KeyFields     []reflect.StructField
	keyCodeGen    keyCodeGenerator
	keyStmt       *jen.Statement
}

func (g JenHelper) GenerateKeyCode(interfaceImport string) jen.Code {
	return g.keyCodeGen.Generate(interfaceImport)
}

type Operation struct {
	slug string
}

func (o Operation) String() string {
	return o.slug
}

var (
	OperationFindByKey = Operation{"findByKey"}
	OperationsCrud     = genlib.NewFrozenSet(OperationFindByKey)
	OperationSearch    = Operation{"search"}
)
