package gorm

import (
	"github.com/activatedio/datainfra/genlib"
	"github.com/dave/jennifer/jen"
)

func NewSearchHandlerEntries() *genlib.HandlerEntries {
	return genlib.NewHandlerEntries().AddStatementHandler(&Ctor{}, func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {
		return s.Add(jen.Commentf("implements the SearchHandler interface."))
	})
}
