package gorm

import (
	"reflect"

	"github.com/activatedio/datainfra/genlib/data"
	"github.com/dave/jennifer/jen"
	"github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
)

const (
	// BaseKeyId is the base key identifier used for accessing key fields.
	BaseKeyId = "key" //nolint:revive // name is okay
)

// pl is an instance of pluralize.Client used for pluralizing and singularizing words in the application.
var (
	pl = pluralize.NewClient()
)

// Key represents a struct with a Name and Type, primarily used for defining key fields in code generation.
type Key struct {
	Accessor jen.Code
	Type     jen.Code
	Name     string
}

// JenHelper represents a helper structure for managing data objects with metadata, table names, and keys.
type JenHelper struct {
	data.JenHelper
	TablePrefix      string
	TableName        string
	Keys             []Key
	ContextScopeCode jen.Code
}

// GetGormJenHelper transforms a data.Entry into a JenHelper enriched with keys and a pluralized table name.
func GetGormJenHelper(entry *data.Entry) JenHelper {
	jh := entry.GetJenHelper()

	var keys []Key

	t := jh.KeyField.Type

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
			Name:     jh.KeyField.Name,
		})
	}

	tableName := pl.Plural(strcase.ToSnake(jh.StructName))
	var csc jen.Code

	i := data.GetImplementation[Implementation](entry)
	if i != nil {
		if i.TableName != "" {
			tableName = i.TableName
		}
		csc = i.ContextScopeCode
	}

	return JenHelper{
		JenHelper:        jh,
		Keys:             keys,
		TablePrefix:      strcase.ToSnake(jh.StructName),
		TableName:        tableName,
		ContextScopeCode: csc,
	}
}
