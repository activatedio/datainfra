//go:build !windows

package gorm

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive advisory lock on f, blocking until it is
// available, and returns the release function.
func lockFile(f *os.File) (func(), error) {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	}, nil
}
