package gorm_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
)

// The lock's whole job is that two holders never overlap. Migrations rely on
// it for the same reason setup's CREATE/DROP DATABASE does: YugabyteDB bumps
// a cluster-wide catalog version on DDL, so overlapping holders is exactly
// the SQLSTATE 40001 this exists to prevent.
func TestWithDDLLockSerializes(t *testing.T) {
	cases := map[string]struct {
		host    string
		port    int
		sameKey bool
	}{
		"holders of the same target never overlap": {host: "lock-test", port: 5433, sameKey: true},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			const workers = 8

			var (
				inside  atomic.Int32
				overlap atomic.Bool
				runs    atomic.Int32
				wg      sync.WaitGroup
			)

			for range workers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					require.NoError(t, datagorm.WithDDLLock(v.host, v.port, func() error {
						if inside.Add(1) > 1 {
							overlap.Store(true)
						}
						// Widen the window a real DDL statement would occupy.
						time.Sleep(5 * time.Millisecond)
						inside.Add(-1)
						runs.Add(1)
						return nil
					}))
				}()
			}
			wg.Wait()

			require.False(t, overlap.Load(), "two holders were inside the lock at once")
			require.EqualValues(t, workers, runs.Load(), "every worker must run")
		})
	}
}

// Different targets must not wait on each other — the lock is keyed by
// host:port so an unrelated server is never serialized behind this one.
func TestWithDDLLockIsPerTarget(t *testing.T) {
	release := make(chan struct{})
	held := make(chan struct{})
	done := make(chan struct{})

	go func() {
		_ = datagorm.WithDDLLock("target-a", 5433, func() error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held

	go func() {
		require.NoError(t, datagorm.WithDDLLock("target-b", 5433, func() error { return nil }))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a different target blocked on this one's lock")
	}
	close(release)
}

// A failing fn must release the lock, or one bad migration wedges every
// later one behind it.
func TestWithDDLLockReleasesOnError(t *testing.T) {
	boom := errBoom{}
	require.ErrorIs(t, datagorm.WithDDLLock("target-err", 5433, func() error { return boom }), boom)

	done := make(chan struct{})
	go func() {
		require.NoError(t, datagorm.WithDDLLock("target-err", 5433, func() error { return nil }))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the lock was not released after fn returned an error")
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }
