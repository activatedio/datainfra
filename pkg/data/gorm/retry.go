package gorm

import (
	"errors"
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

// IsSerializationFailure reports whether err carries the retryable
// SQLSTATE 40001 (serialization_failure / YugabyteDB read-restart) code.
func IsSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40001"
}

// ExecWithSerializationRetry runs the statement through db.Exec, retrying
// with linear backoff while the failure is a retryable serialization error
// (SQLSTATE 40001). A statement aborted with 40001 has not been applied, so
// re-issuing it verbatim is safe.
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
