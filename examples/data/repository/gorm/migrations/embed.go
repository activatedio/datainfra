// Package migrations contains migration files
package migrations

import "embed"

// Files contains migration files
//
//go:embed *.sql
var Files embed.FS
