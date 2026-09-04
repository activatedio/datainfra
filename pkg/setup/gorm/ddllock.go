package gorm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
)

// Serializing CREATE/DROP DATABASE
//
// CREATE DATABASE and DROP DATABASE are global-impact DDL on YugabyteDB:
// they bump every database's catalog version, and they are not
// transactional. Issued concurrently they conflict, and the conflict does
// not fail cleanly — a read restart (SQLSTATE 40001) can be reported after
// the DocDB keyspace has already been created while the catalog row is
// rolled back. What is left is a keyspace with no pg_database row, which
// makes every retry of that name fail with "Keyspace 'x' already exists"
// (SQLSTATE XX000) and is not reaped on any timescale a caller can wait
// out. On older builds the same workload could also segfault the tserver.
//
// Measured with a 12-worker, 3-round create/drop stress test against
// YugabyteDB 2026.1.1.1-b2: 8 of 36 setups failed concurrently, 0 of 8
// serialized. So this serializes the two statements rather than trying to
// recover from their collision.
//
// The lock is a file lock, which serializes every process on the machine —
// the case that matters, since `go test ./...` runs one process per package
// against one database server. It is keyed by target host and port so
// unrelated servers do not wait on each other, and it is held only around
// the DDL statement itself, so everything else still runs in parallel.
//
// A file lock deliberately does not span machines. Cross-machine bring-up
// (several replicas of one service starting at once) is already handled by
// the duplicate-object checks in createDatabase: the loser of that race
// finds the database present, which is the outcome it wanted.

// inProcessDDLLocks guards the DDL lock file per target within this process.
// The file lock alone is not enough: flock is advisory per open file
// description, and two goroutines in one process sharing a descriptor would
// not block each other.
var (
	inProcessDDLLocksMu sync.Mutex
	inProcessDDLLocks   = map[string]*sync.Mutex{}
)

func inProcessDDLLock(key string) *sync.Mutex {
	inProcessDDLLocksMu.Lock()
	defer inProcessDDLLocksMu.Unlock()
	m, ok := inProcessDDLLocks[key]
	if !ok {
		m = &sync.Mutex{}
		inProcessDDLLocks[key] = m
	}
	return m
}

// ddlLockPath names the lock file for a target. The target is hashed rather
// than interpolated so a host name cannot produce a surprising path.
func ddlLockPath(host string, port int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", host, port)))
	return filepath.Join(os.TempDir(), "datainfra-dbddl-"+hex.EncodeToString(sum[:8])+".lock")
}

// withDatabaseDDLLock runs fn while holding the machine-wide lock for the
// owner target.
//
// A lock that cannot be taken is logged and skipped rather than failing the
// operation: the lock is a serialization aid, not a correctness barrier, and
// a read-only or unusual TMPDIR should not stop a database from being
// created. The caller's own duplicate/conflict handling remains the
// backstop.
func (g *gormSetup) withDatabaseDDLLock(fn func() error) error {

	key := fmt.Sprintf("%s:%d", g.ownerConfig.Host, g.ownerConfig.Port)

	mu := inProcessDDLLock(key)
	mu.Lock()
	defer mu.Unlock()

	path := ddlLockPath(g.ownerConfig.Host, g.ownerConfig.Port)

	// The path is ddlLockPath's output — os.TempDir plus a hash of host:port —
	// and never caller-supplied.
	//nolint:gosec // G304: path is not caller-controlled, see above.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		log.Warn().Err(err).Str("path", path).
			Msg("could not open the database-DDL lock file; proceeding without cross-process serialization")
		return fn()
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			log.Debug().Err(cerr).Msg("could not close the database-DDL lock file")
		}
	}()

	unlock, err := lockFile(f)
	if err != nil {
		log.Warn().Err(err).Str("path", path).
			Msg("could not take the database-DDL lock; proceeding without cross-process serialization")
		return fn()
	}
	defer unlock()

	return fn()
}
