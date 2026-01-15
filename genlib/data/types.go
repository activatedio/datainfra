package data

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/activatedio/gen"
	"github.com/dave/jennifer/jen"
)

const (
	// BaseKeyId is the base key identifier used for accessing key fields.
	BaseKeyId = "key" //nolint:revive // name is okay
)

// Entry represents a data descriptor containing type metadata, supported operations, and implementation-specific details.
type Entry struct {
	Type reflect.Type
	// Implementations are implementation-specific parameters
	Implementations []any
}

func getKey(t reflect.Type) *reflect.StructField {
	var res *reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		dt := ParseTag(f.Tag.Get("data"))

		var tmp *reflect.StructField

		if dt.IsKey {
			tmp = &f
		} else if f.Anonymous && f.Type.Kind() == reflect.Struct {
			tmp = getKey(f.Type)
		}

		if tmp != nil && res != nil {
			panic("multiple key fields found for " + t.Name())
		}
		if tmp == nil {
			continue
		}
		res = tmp
	}
	return res
}

// GetJenHelper generates a JenHelper object for the Entry, which includes interface and struct metadata and key field analysis.
func (e Entry) GetJenHelper() JenHelper {

	res := JenHelper{
		InterfaceName: fmt.Sprintf("%sRepository", e.Type.Name()),
		StructName:    e.Type.Name(),
	}

	res.StructType = jen.Qual(e.Type.PkgPath(), e.Type.Name())
	res.KeyField = getKey(e.Type)

	if res.KeyField == nil {
		panic("key field not set for " + e.Type.Name())
	}

	res.keyCodeGen = func() jen.Code {
		return jen.Qual(res.KeyField.Type.PkgPath(), res.KeyField.Type.Name())
	}

	var keys []Key

	t := res.KeyField.Type

	switch t.Kind() {
	case reflect.Struct:
		// We can only have one key per field from the key type
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			keys = append(keys, Key{
				Accessor: jen.Id(BaseKeyId).Dot(f.Name),
				Type:     jen.Qual(f.Type.PkgPath(), f.Type.Name()),
				Name:     f.Name,
			})
		}
	case reflect.Ptr:
		panic("key can't be a pointer type")
	default:
		keys = append(keys, Key{
			Accessor: jen.Id(BaseKeyId),
			Type:     jen.Qual(t.PkgPath(), t.Name()),
			Name:     res.KeyField.Name,
		})
	}

	res.Keys = keys

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

// Key represents a struct with a Name and Type, primarily used for defining key fields in code generation.
type Key struct {
	Accessor jen.Code
	Type     jen.Code
	Name     string
}

// JenHelper is a structure designed to aid in generating Go code and managing metadata for data objects.
type JenHelper struct {
	InterfaceName string
	StructType    jen.Code
	StructName    string
	// Can only have one key field
	KeyField   *reflect.StructField
	Keys       []Key
	keyCodeGen func() jen.Code
	keyStmt    *jen.Statement
}

// GenerateKeyCode generates a key code for the given interface import using the keyCodeGen generator field of JenHelper.
// Returns a jen.Code instance representing the generated code for the provided interface import.
func (g JenHelper) GenerateKeyCode() jen.Code {
	return g.keyCodeGen()
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
	OperationsCrud = gen.NewFrozenSet(
		OperationFindByKey, OperationList, OperationCreate, OperationUpdate, OperationDelete,
	)
)
