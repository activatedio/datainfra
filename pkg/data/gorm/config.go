package gorm

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
