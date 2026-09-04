package gorm_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
)

func TestIsSerializationFailure(t *testing.T) {

	cases := map[string]struct {
		arrange func() error
		assert  func(t *testing.T, got bool)
	}{
		"pg serialization failure": {
			arrange: func() error {
				return &pgconn.PgError{Code: "40001", Message: "Restart read required"}
			},
			assert: func(t *testing.T, got bool) {
				assert.True(t, got)
			},
		},
		"wrapped pg serialization failure": {
			arrange: func() error {
				return fmt.Errorf("migration %q failed: %w", "main",
					&pgconn.PgError{Code: "40001", Message: "Restart read required"})
			},
			assert: func(t *testing.T, got bool) {
				assert.True(t, got)
			},
		},
		// YugabyteDB reports the condition postgres calls 40001 under its
		// own code. Observed aborting a DROP DATABASE during concurrent test
		// teardown, where the 40001-only check let it through unretried.
		"yugabyte concurrent-update conflict": {
			arrange: func() error {
				return &pgconn.PgError{Code: "YB003",
					Message: "could not serialize access due to concurrent update"}
			},
			assert: func(t *testing.T, got bool) {
				assert.True(t, got)
			},
		},
		"other pg error": {
			arrange: func() error {
				return &pgconn.PgError{Code: "42P01", Message: "relation does not exist"}
			},
			assert: func(t *testing.T, got bool) {
				assert.False(t, got)
			},
		},
		"non-pg error": {
			arrange: func() error {
				return errors.New("dial tcp: connection refused")
			},
			assert: func(t *testing.T, got bool) {
				assert.False(t, got)
			},
		},
		"nil error": {
			arrange: func() error {
				return nil
			},
			assert: func(t *testing.T, got bool) {
				assert.False(t, got)
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			v.assert(t, gorm.IsSerializationFailure(v.arrange()))
		})
	}
}

// IsNamespaceExists keys on a message because XX000 is postgres' generic
// internal_error and YugabyteDB offers nothing more specific. These cases pin
// which messages count, so a broadening of the match is a deliberate edit.
func TestIsNamespaceExists(t *testing.T) {

	cases := map[string]struct {
		err  error
		want bool
	}{
		"yb keyspace exists": {
			err: &pgconn.PgError{Code: "XX000",
				Message: "Keyspace 'stress_1_2_3' already exists"},
			want: true,
		},
		"yb namespace exists": {
			err: &pgconn.PgError{Code: "XX000",
				Message: "Namespace already exists"},
			want: true,
		},
		"wrapped": {
			err: fmt.Errorf("creating database: %w",
				&pgconn.PgError{Code: "XX000", Message: "Keyspace 'x' already exists"}),
			want: true,
		},
		// The postgres-level duplicate is a different condition with a
		// different meaning — the database really is there. IsDuplicateObject
		// owns it.
		"postgres duplicate database is not this": {
			err:  &pgconn.PgError{Code: "42P04", Message: `database "x" already exists`},
			want: false,
		},
		// Same code, unrelated failure.
		"other internal error": {
			err:  &pgconn.PgError{Code: "XX000", Message: "Duplicate table"},
			want: false,
		},
		"non-pg error": {
			err:  errors.New("boom"),
			want: false,
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, v.want, gorm.IsNamespaceExists(v.err))
		})
	}
}

func TestIsUndefinedObject(t *testing.T) {

	cases := map[string]struct {
		err  error
		want bool
	}{
		"absent database": {
			err:  &pgconn.PgError{Code: "3D000", Message: `database "x" does not exist`},
			want: true,
		},
		"absent role": {
			err:  &pgconn.PgError{Code: "42704", Message: `role "x" does not exist`},
			want: true,
		},
		"wrapped": {
			err: fmt.Errorf("dropping: %w",
				&pgconn.PgError{Code: "3D000", Message: "does not exist"}),
			want: true,
		},
		"duplicate is not undefined": {
			err:  &pgconn.PgError{Code: "42P04", Message: "already exists"},
			want: false,
		},
		"non-pg error": {
			err:  errors.New("boom"),
			want: false,
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, v.want, gorm.IsUndefinedObject(v.err))
		})
	}
}
