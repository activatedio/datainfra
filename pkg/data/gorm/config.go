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
}
