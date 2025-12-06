// Package migrations contains migration files
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
