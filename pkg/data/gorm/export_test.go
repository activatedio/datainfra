package gorm

import "github.com/activatedio/datainfra/pkg/data"

// TestablePostgresBucketSpec exposes the unexported postgresBucketSpec
// helper to external (_test) packages. The helper has no dependency on
// a live database, so unit tests use it to lock in the SQL fragment
// shapes the histogram template will splice into the generated query.
func TestablePostgresBucketSpec(ri data.HistogramInterval, column string) (string, string, error) {
	return postgresBucketSpec(ri, column)
}

// TestablePostgresAlignExpr exposes the unexported postgresAlignExpr
// helper to external (_test) packages for the same reason.
func TestablePostgresAlignExpr(ri data.HistogramInterval, placeholder string) string {
	return postgresAlignExpr(ri, placeholder)
}
