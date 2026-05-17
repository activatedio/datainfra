package gorm

import (
	"github.com/dave/jennifer/jen"
	"github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"

	"github.com/activatedio/datainfra/genlib/data"
)

// pl is an instance of pluralize.Client used for pluralizing and singularizing words in the application.
var (
	pl = pluralize.NewClient()
)

// JenHelper represents a helper structure for managing data objects with metadata, table names, and keys.
type JenHelper struct {
	data.JenHelper
	TablePrefix      string
	TableName        string
	ContextScopeCode jen.Code
}

// GetGormJenHelper transforms a data.Entry into a JenHelper enriched with keys and a pluralized table name.
func GetGormJenHelper(entry *data.Entry) JenHelper {
	jh := entry.GetJenHelper()

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
		TablePrefix:      strcase.ToSnake(jh.StructName),
		TableName:        tableName,
		ContextScopeCode: csc,
	}
}
