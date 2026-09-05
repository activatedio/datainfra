package gorm

import (
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Serialization-failure retry policy. Databases that abort statements with
// SQLSTATE 40001 and expect the client to retry — YugabyteDB's "Restart
// read required" under concurrent DDL, postgres serialization_failure under
// SERIALIZABLE — would otherwise fail whole setup or migration runs on a
// transient conflict.
const (
	serializationRetryAttempts = 5
	serializationRetryBaseWait = 200 * time.Millisecond
)

// IsSerializationFailure reports whether err carries a retryable
// serialization conflict.
//
// SQLSTATE 40001 is the standard one (serialization_failure, and
// YugabyteDB's "Restart read required"). YB003 is YugabyteDB's own code for
// "could not serialize access due to concurrent update" — the same condition
// vanilla postgres reports as 40001, under a code the 40001 check missed.
// Observed aborting a DROP DATABASE during concurrent test teardown, where it
// surfaced as an unretried hard failure.
func IsSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40001", "YB003":
		return true
	default:
		return false
	}
}

// IsNamespaceExists reports whether err is YugabyteDB complaining that the
// underlying keyspace for a database already exists — "Keyspace 'x' already
// exists", raised at the DocDB layer under SQLSTATE XX000 rather than
// postgres' own 42P04.
//
// This is not the same condition as IsDuplicateObject, and must not be
// treated as one. It is what a CREATE DATABASE sees after an earlier attempt
// hit a read restart: the keyspace was created, the catalog row was rolled
// back, and the orphaned keyspace is cleaned up asynchronously. So the
// database may or may not actually exist from postgres' point of view, and
// the caller has to look rather than assume — see the retry in
// setup/gorm's createDatabase.
//
// XX000 is postgres' generic internal_error, so the message has to be part of
// the test; there is no more specific code to key on.
func IsNamespaceExists(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "XX000" {
		return false
	}
	m := strings.ToLower(pgErr.Message)
	return strings.Contains(m, "already exists") &&
		(strings.Contains(m, "keyspace") || strings.Contains(m, "namespace"))
}

// IsUndefinedObject reports whether err says the object being dropped is not
// there: SQLSTATE 3D000 (invalid_catalog_name — an absent database) or 42704
// (undefined_object, roles among them).
//
// This is the mirror of IsDuplicateObject and exists for the same reason. A
// DROP retried after a serialization conflict can find the first attempt
// already applied, and "it is gone" is the outcome the drop was after.
func IsUndefinedObject(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "3D000", "42704":
		return true
	default:
		return false
	}
}

// IsDuplicateObject reports whether err says the object being created
// already exists: SQLSTATE 42P04 (duplicate_database), 42710
// (duplicate_object — roles among them) or 42P06 (duplicate_schema).
//
// These are what the loser of a provisioning race sees. Setup's exist-checks
// are check-then-create, so two processes bringing up the same database
// target concurrently — replicas of one service, or two services sharing a
// database — can both observe "absent" and both issue CREATE; exactly one
// wins, and for the other the object now exists, which is the outcome setup
// was after all along.
func IsDuplicateObject(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "42P04", "42710", "42P06":
		return true
	default:
		return false
	}
}

// ExecWithSerializationRetry runs the statement through db.Exec, retrying
// with linear backoff while the failure is a retryable serialization error
// (see IsSerializationFailure).
//
// For ordinary DML and for DDL inside a transaction, an aborted statement has
// not been applied and re-issuing it verbatim is safe. That is NOT true of
// CREATE/DROP DATABASE on YugabyteDB, which are not transactional: a read
// restart can be reported after the keyspace has already been created or
// removed, so a blind retry sees "already exists" or "does not exist". Those
// two callers must check the resulting state rather than trust this
// function's return — setup/gorm's createDatabase and dropDatabase do.
func ExecWithSerializationRetry(db *gorm.DB, stmt string, args ...any) error {
	var err error
	for attempt := 1; attempt <= serializationRetryAttempts; attempt++ {
		if err = db.Exec(stmt, args...).Error; err == nil || !IsSerializationFailure(err) {
			return err
		}
		log.Warn().Int("attempt", attempt).Err(err).Msg("retrying statement after serialization failure")
		time.Sleep(time.Duration(attempt) * serializationRetryBaseWait)
	}
	return err
}
