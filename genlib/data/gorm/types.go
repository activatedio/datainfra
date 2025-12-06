package gorm

import (
	"github.com/activatedio/datainfra/genlib/data"
	"github.com/dave/jennifer/jen"
	"github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
)

// pl is an instance of pluralize.Client used for pluralizing and singularizing words in the application.
var (
	pl = pluralize.NewClient()
)

// Key represents a struct with a Name and Type, primarily used for defining key fields in code generation.
type Key struct {
	Name string
	Type jen.Code
}

// JenHelper represents a helper structure for managing data objects with metadata, table names, and keys.
type JenHelper struct {
	data.JenHelper
	TablePrefix string
	TableName   string
	Keys        []Key
}

// GetGormJenHelper transforms a data.Entry into a JenHelper enriched with keys and a pluralized table name.
func GetGormJenHelper(entry *data.Entry) JenHelper {
	jh := entry.GetJenHelper()

	keys := make([]Key, len(jh.KeyFields))

	for i, k := range jh.KeyFields {

		keys[i] = Key{
			Name: strcase.ToSnake(k.Name),
			Type: jen.Qual(k.PkgPath, k.Type.String()),
		}
	}

	return JenHelper{
		JenHelper:   jh,
		Keys:        keys,
		TablePrefix: strcase.ToSnake(jh.StructName),
		TableName:   pl.Plural(strcase.ToSnake(jh.StructName)),
	}
}
