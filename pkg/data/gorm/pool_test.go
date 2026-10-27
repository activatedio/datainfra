package gorm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// recordingPool records each pool setter applyPool calls.
type recordingPool struct {
	calls map[string]any
}

func (r *recordingPool) SetMaxIdleConns(n int)              { r.calls["maxIdleConns"] = n }
func (r *recordingPool) SetMaxOpenConns(n int)              { r.calls["maxOpenConns"] = n }
func (r *recordingPool) SetConnMaxIdleTime(d time.Duration) { r.calls["connMaxIdleTime"] = d }
func (r *recordingPool) SetConnMaxLifetime(d time.Duration) { r.calls["connMaxLifetime"] = d }

// A pool field is applied only when the Config names it, so a nil field
// keeps database/sql's default; SQLite always gets one open connection.
func TestApplyPool(t *testing.T) {
	intPtr := func(n int) *int { return &n }
	durPtr := func(d time.Duration) *time.Duration { return &d }

	cases := map[string]struct {
		config   Config
		expected map[string]any
	}{
		"postgres nothing set": {
			config:   Config{Dialect: DialectPostgres},
			expected: map[string]any{},
		},
		"postgres all set": {
			config: Config{
				Dialect:         DialectPostgres,
				MaxIdleConns:    intPtr(10),
				MaxOpenConns:    intPtr(25),
				ConnMaxIdleTime: durPtr(5 * time.Minute),
				ConnMaxLifetime: durPtr(30 * time.Minute),
			},
			expected: map[string]any{
				"maxIdleConns":    10,
				"maxOpenConns":    25,
				"connMaxIdleTime": 5 * time.Minute,
				"connMaxLifetime": 30 * time.Minute,
			},
		},
		"postgres idle only": {
			config:   Config{Dialect: DialectPostgres, MaxIdleConns: intPtr(4)},
			expected: map[string]any{"maxIdleConns": 4},
		},
		"sqlite nothing set": {
			config:   Config{Dialect: DialectSqlite},
			expected: map[string]any{"maxOpenConns": 1},
		},
		"sqlite ignores max open": {
			config:   Config{Dialect: DialectSqlite, MaxOpenConns: intPtr(25), ConnMaxLifetime: durPtr(time.Minute)},
			expected: map[string]any{"maxOpenConns": 1, "connMaxLifetime": time.Minute},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			p := &recordingPool{calls: map[string]any{}}
			applyPool(p, &v.config)
			assert.Equal(t, v.expected, p.calls)
		})
	}
}
