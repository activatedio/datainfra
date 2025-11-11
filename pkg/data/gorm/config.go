package gorm

type GormConfig struct {
	Dialect                  string
	EnableDefaultTransaction bool
	EnableSQLLogging         bool
	Host                     string
	Port                     int
	Username                 string
	Password                 string
	Name                     string
}
