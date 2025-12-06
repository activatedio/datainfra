package data

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/activatedio/datainfra/genlib"
	"github.com/dave/jennifer/jen"
)

// Entry represents a data descriptor containing type metadata, supported operations, and implementation-specific details.
type Entry struct {
	Type reflect.Type
	// Implementations are implementation-specific parameters
	Implementations []any
}

// GetJenHelper generates a JenHelper object for the Entry, which includes interface and struct metadata and key field analysis.
func (e Entry) GetJenHelper() JenHelper {

	res := JenHelper{
		InterfaceName: fmt.Sprintf("%sRepository", e.Type.Name()),
		StructName:    e.Type.Name(),
	}

	res.StructType = jen.Qual(e.Type.PkgPath(), e.Type.Name())

	for i := 0; i < e.Type.NumField(); i++ {
		f := e.Type.Field(i)
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
		keyTypeName := fmt.Sprintf("%sKey", e.Type.Name())
		res.keyCodeGen = &localKeyCodeGenerator{
			id: keyTypeName,
		}

		fs := &jen.Statement{}

		fs.Add(jen.Commentf("%s is the key for %s", keyTypeName, e.Type.Name()))

		for _, key := range res.KeyFields {
			fs.Add(jen.Id(key.Name).Qual(key.Type.PkgPath(), key.Type.Name()))
		}

		res.keyStmt = jen.Type().Id(keyTypeName).Struct(*fs...)
	}

	return res
}

// Tag represents metadata information, where IsKey indicates whether the tag is a key.
type Tag struct {
	IsKey bool
}

// ParseTag parses a given tag string and returns a Tag object with its properties set based on the parsed content.
// If the tag contains "key", the IsKey property of the returned Tag is set to true.
func ParseTag(tag string) Tag {

	t := Tag{}

	for _, v := range strings.Split(tag, ",") {
		if v == "key" {
			t.IsKey = true
		}
	}

	return t
}

// keyCodeGenerator defines behavior for generating key code based on import context and data interface.
type keyCodeGenerator interface {
	// Generate generates a key code based on the relative import of the data interface type. For a local generation
	// this import will be blank.  The generator generates the code for the key, which may or may not use the
	// provided interface type depending on if the key type is local or not
	Generate(interfaceImport string) jen.Code
}

// fixedKeyCodeGenerator is a type responsible for generating fixed key codes using a predefined `jen.Code` instance.
type fixedKeyCodeGenerator struct {
	code jen.Code
}

// Generate returns a pre-defined jen.Code object associated with the fixedKeyCodeGenerator instance.
func (f *fixedKeyCodeGenerator) Generate(interfaceImport string) jen.Code {
	return f.code
}

// localKeyCodeGenerator generates code for a locally scoped key type with a specified identifier.
type localKeyCodeGenerator struct {
	id string
}

// Generate constructs a code representation of the identifier, optionally qualifying it with the given import path.
func (l *localKeyCodeGenerator) Generate(interfaceImport string) jen.Code {
	if interfaceImport == "" {
		return jen.Id(l.id)
	} else {
		return jen.Qual(interfaceImport, l.id)
	}
}

// JenHelper is a structure designed to aid in generating Go code and managing metadata for data objects.
type JenHelper struct {
	InterfaceName string
	StructType    jen.Code
	StructName    string
	KeyFields     []reflect.StructField
	keyCodeGen    keyCodeGenerator
	keyStmt       *jen.Statement
}

// GenerateKeyCode generates a key code for the given interface import using the keyCodeGen generator field of JenHelper.
// Returns a jen.Code instance representing the generated code for the provided interface import.
func (g JenHelper) GenerateKeyCode(interfaceImport string) jen.Code {
	return g.keyCodeGen.Generate(interfaceImport)
}

// Operation represents a specific action or operation identified by a unique slug.
type Operation struct {
	slug string
}

// String returns the string representation of an Operation by providing its slug value.
func (o Operation) String() string {
	return o.slug
}

var (
	// OperationFindByKey defines an operation for finding items by a key.
	OperationFindByKey = Operation{"findByKey"}
	// OperationList defines an operation for listing items.
	OperationList = Operation{"list"}
	// OperationCreate defines an operation for creating items.
	OperationCreate = Operation{"create"}
	// OperationUpdate defines an operation for updating items.
	OperationUpdate = Operation{"update"}
	// OperationDelete defines an operation for deleting items.
	OperationDelete = Operation{"delete"}
	// OperationsCrud represents a frozen set containing all CRUD operations.
	OperationsCrud = genlib.NewFrozenSet(
		OperationFindByKey, OperationList, OperationCreate, OperationUpdate, OperationDelete,
	)
)
