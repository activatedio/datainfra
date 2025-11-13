package gorm

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/activatedio/datainfra/pkg/data"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	GormDialectPsql   = "psql"
	GormDialectSqlite = "sqlite"
)

// NewDB creates and configures a new Gorm database instance based on the provided DBConfig.
func NewDB(config *GormConfig) (*gorm.DB, error) {

	var dialector gorm.Dialector

	switch config.Dialect {
	case GormDialectPsql:
		dialector = postgres.New(postgres.Config{
			DSN: fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
				config.Host, config.Port, config.Username, config.Password, config.Name),
		})
	case GormDialectSqlite:
		dialector = sqlite.Open(fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", config.Name))
	default:
		panic("unexpected dialect " + config.Dialect)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		SkipDefaultTransaction: !config.EnableDefaultTransaction,
		Logger: func() logger.Interface {
			if config.EnableSQLLogging {
				return logger.Default.LogMode(logger.Info)
			}
			return nil
		}(),
	})

	if err != nil {
		return nil, err
	}

	switch config.Dialect {
	case GormDialectPsql:
	case GormDialectSqlite:
		var _db *sql.DB
		_db, err = db.DB()
		if err != nil {
			return nil, err
		}
		_db.SetMaxOpenConns(1)
	default:
		panic("unexpected dialect " + config.Dialect)
	}

	return db, err

}

type contextKey struct {
	name string
}

var dbKey = contextKey{
	name: "db",
}

// WithDB returns a new context with the provided *gorm.DB instance stored in it under a specific key.
func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, dbKey, db)
}

type contextBuilder struct {
	rootDB *gorm.DB
}

// Build injects the rootDB from the contextBuilder into the given context and returns the updated context.
func (c *contextBuilder) Build(ctx context.Context) context.Context {
	return WithDB(ctx, c.rootDB)
}

// NewContextBuilder initializes and returns a ContextBuilder with the provided database connection.
func NewContextBuilder(db *gorm.DB) data.ContextBuilder {
	return &contextBuilder{
		rootDB: db,
	}
}

// GetDB retrieves the *gorm.DB instance from the provided context.
// Panics if the database instance is not found in the context.
func GetDB(ctx context.Context) *gorm.DB {

	tx, ok := ctx.Value(dbKey).(*gorm.DB)
	if !ok {
		panic("DB not in context")
	}
	return tx
}
