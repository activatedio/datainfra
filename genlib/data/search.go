package data

import (
	"github.com/activatedio/datainfra/genlib"
	"github.com/dave/jennifer/jen"
)

func NewSearchHandlerEntries() *genlib.HandlerEntries {
	return genlib.NewHandlerEntries().AddStatementHandler(&InterfaceMethods{}, func(s *jen.Statement, r genlib.Registry, entry any) *jen.Statement {
		return s.Add(jen.Commentf("Need to add search methods here"))
	})
}
