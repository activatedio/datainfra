package gorm

import "github.com/dave/jennifer/jen"

// Implementation defines the configuration for a gorm data access implementation
type Implementation struct {
	// TableName allows overriding of the table name
	TableName        string
	ContextScopeCode jen.Code
}
