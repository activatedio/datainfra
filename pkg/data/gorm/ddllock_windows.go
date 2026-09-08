//go:build windows

package gorm

import "os"

// lockFile is a no-op on Windows: there is no flock, and the concurrent-DDL
// problem this serializes is a YugabyteDB-on-CI concern. Callers treat a
// missing lock as "proceed", so the duplicate/conflict handling in
// createDatabase remains the backstop.
func lockFile(_ *os.File) (func(), error) {
	return func() {}, nil
}
