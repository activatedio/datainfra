package data

import (
	"github.com/dave/jennifer/jen"
	"github.com/gertd/go-pluralize"
)

var (
	QualCtx = jen.Qual("context", "Context")
	IdError = jen.Id("error")

	Pl = pluralize.NewClient()
)
