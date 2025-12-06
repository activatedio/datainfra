// Package gorm contains gorm related test data
package gorm

import "embed"

//go:embed *.sql
var Files embed.FS
