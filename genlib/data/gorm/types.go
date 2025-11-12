package gorm

import (
	"github.com/activatedio/datainfra/genlib/data"
	"github.com/dave/jennifer/jen"
	"github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
)

var (
	pl = pluralize.NewClient()
)

type Key struct {
	Name string
	Type jen.Code
}

type JenHelper struct {
	data.JenHelper
	TableName string
	Keys      []Key
}

func GetGormJenHelper(entry *data.Entry) JenHelper {
	jh := entry.GetJenHelper()

	var keys []Key

	for _, k := range jh.KeyFields {

		keys = append(keys, Key{
			Name: strcase.ToSnake(k.Name),
			Type: jen.Qual(k.PkgPath, k.Type.String()),
		})
	}

	return JenHelper{
		JenHelper: jh,
		Keys:      keys,
		TableName: pl.Plural(strcase.ToSnake(jh.StructName)),
	}
}
