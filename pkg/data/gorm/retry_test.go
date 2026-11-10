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

// TestIsTransientSchemaChange pins kit#937's classifier: the faults a schema
// change under a running statement leaves, which a new transaction clears.
func TestIsTransientSchemaChange(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"serialization failure":      {err: &pgconn.PgError{Code: "40001", Message: "Restart read required"}, want: true},
		"yugabyte concurrent update": {err: &pgconn.PgError{Code: "YB003"}, want: true},
		"yugabyte schema version mismatch, wrapped": {
			err:  fmt.Errorf("find issuer: %w", &pgconn.PgError{Code: "40001", Message: "schema version mismatch for table 0000400b: expected 12, got 11"}),
			want: true,
		},
		"yugabyte schema version mismatch as an internal error": {
			err:  &pgconn.PgError{Code: "XX000", Message: "Schema version mismatch for table 0000400b"},
			want: true,
		},
		"cached plan must not change result type": {
			err:  &pgconn.PgError{Code: "0A000", Message: "cached plan must not change result type"},
			want: true,
		},
		"another feature-not-supported": {err: &pgconn.PgError{Code: "0A000", Message: "cannot do that"}},
		"another internal error":        {err: &pgconn.PgError{Code: "XX000", Message: "boom"}},
		"a constraint violation":        {err: &pgconn.PgError{Code: "23505"}},
		"not a postgres error":          {err: errors.New("dial tcp: connection refused")},
	}
	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, v.want, gorm.IsTransientSchemaChange(v.err))
		})
	}
}
