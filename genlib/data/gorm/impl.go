package gorm

import "github.com/dave/jennifer/jen"

// Implementation defines the configuration for a gorm data access implementation
type Implementation struct {
	// TableName allows overriding of the table name
	TableName         string
	ContextScopeCode  jen.Code
	CustomFindBuilder jen.Code
	// Histogram opts the entry into a real Postgres-backed SearchHistogram
	// method on the generated repository. nil leaves the existing
	// Unimplemented stub in place (the SQLite backend always returns that
	// stub error at runtime even when Histogram is set).
	Histogram *Histogram
}

// Histogram configures the Postgres SearchHistogram method emitted on the
// generated gorm repository. DateColumn is the SQL column the date
// histogram aggregates on (e.g. "created_at").
type Histogram struct {
	DateColumn string
}
