package gorm

import (
	"time"

	"github.com/rs/zerolog"
)

// Config defines the configuration for a gorm database connection.
type Config struct {
	Dialect                  string
	EnableDefaultTransaction bool
	EnableSQLLogging         bool
	Host                     string
	Port                     int
	Username                 string
	Password                 string
	Name                     string
	// The pool fields are applied when set and leave database/sql's default
	// when nil (unlimited open connections, 2 idle, no idle or lifetime
	// limit). A caller with a burst of concurrent reads wants all four: with
	// the default 2 idle connections every request past the second opens a
	// new session and closes it again. SQLite always runs one open
	// connection, whatever MaxOpenConns says.
	MaxIdleConns    *int
	MaxOpenConns    *int
	ConnMaxIdleTime *time.Duration
	ConnMaxLifetime *time.Duration
	// SSLMode is libpq's sslmode (disable, require, verify-ca, verify-full).
	// Empty leaves the driver default. It is connection configuration rather
	// than process environment (PGSSLMODE) so two pools in one process can
	// disagree — an in-cluster database and a managed one, say.
	SSLMode string
	// SSLRootCert is the path to the CA bundle that verifies the server, for
	// the verify-* modes.
	SSLRootCert string
	// Schema is the Postgres schema the connection works in. It is sent as
	// the search_path startup parameter, so every pooled connection resolves
	// unqualified names there, goose's version table included, and a
	// service sharing a database with others sees only its own tables. Empty
	// is the server's default (public). A lower-case SQL identifier. Ignored
	// by sqlite, where a database is a file of its own.
	Schema string
}

// redactedPassword stands in for a password a Config is logged with.
const redactedPassword = "[redacted]"

// MarshalZerologObject logs a Config without its password: a log line is
// read far more widely than the secret it came from. Log one with
// zerolog's Object, never Interface, which marshals every field.
func (c Config) MarshalZerologObject(e *zerolog.Event) {
	e.Str("dialect", c.Dialect).Str("host", c.Host).Int("port", c.Port).Str("username", c.Username).
		Str("name", c.Name).Str("schema", c.Schema).Str("sslMode", c.SSLMode).Bool("sqlLogging", c.EnableSQLLogging)
	if c.Password != "" {
		e.Str("password", redactedPassword)
	}
	if c.MaxIdleConns != nil {
		e.Int("maxIdleConns", *c.MaxIdleConns)
	}
	if c.MaxOpenConns != nil {
		e.Int("maxOpenConns", *c.MaxOpenConns)
	}
	if c.ConnMaxIdleTime != nil {
		e.Dur("connMaxIdleTime", *c.ConnMaxIdleTime)
	}
	if c.ConnMaxLifetime != nil {
		e.Dur("connMaxLifetime", *c.ConnMaxLifetime)
	}
}

// Redacted is the Config with its password replaced, for anything that must
// print a whole Config.
func (c Config) Redacted() Config {
	if c.Password != "" {
		c.Password = redactedPassword
	}
	return c
}
