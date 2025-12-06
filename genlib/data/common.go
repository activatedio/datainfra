package data

import (
	"github.com/dave/jennifer/jen"
	"github.com/gertd/go-pluralize"
)

var (
	// QualCtx is a qualified context.Context type.
	QualCtx = jen.Qual("context", "Context")
	// IDError is a qualified error type.
	IDError = jen.Id("error")

	// Pl is a pluralizer.
	Pl = pluralize.NewClient()
)
