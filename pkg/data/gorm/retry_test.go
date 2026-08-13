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
