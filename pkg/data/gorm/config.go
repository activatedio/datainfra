package gorm

import "github.com/rs/zerolog"

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
	MaxIdleConns             *int
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
}

// Redacted is the Config with its password replaced, for anything that must
// print a whole Config.
func (c Config) Redacted() Config {
	if c.Password != "" {
		c.Password = redactedPassword
	}
	return c
}
