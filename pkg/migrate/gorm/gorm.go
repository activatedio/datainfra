package gorm

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/migrate"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

// MigratorData represents a migration configuration including its name, file system, and relative path.
type MigratorData struct {
	Name string
	FS   fs.FS
	Path string
}

// migrator handles database migration processes using the provided configuration and migration data.
type migrator struct {
	config *datagorm.Config
	data   []MigratorData
}

var dialectMap = map[string]goose.Dialect{
	"postgres":   goose.DialectPostgres,
	"pgx":        goose.DialectPostgres,
	"mysql":      goose.DialectMySQL,
	"sqlite3":    goose.DialectSQLite3,
	"sqlite":     goose.DialectSQLite3,
	"mssql":      goose.DialectMSSQL,
	"azuresql":   goose.DialectMSSQL,
	"sqlserver":  goose.DialectMSSQL,
	"redshift":   goose.DialectRedshift,
	"tidb":       goose.DialectTiDB,
	"clickhouse": goose.DialectClickHouse,
	"ydb":        goose.DialectYdB,
	"turso":      goose.DialectTurso,
	"starrocks":  goose.DialectStarrocks,
}

// dialectFromString maps a dialect string to the goose Dialect constant.
func dialectFromString(s string) (goose.Dialect, error) {
	if d, ok := dialectMap[s]; ok {
		return d, nil
	}
	return "", fmt.Errorf("%q: unknown dialect", s)
}

// Migrate executes database migrations using the configuration and migration data defined in the migrator instance.
func (m *migrator) Migrate(ctx context.Context) error {

	gdb, err := datagorm.NewDB(m.config)
	if err != nil {
		return err
	}

	db, err := gdb.DB()
	if err != nil {
		return err
	}

	dialect, err := dialectFromString(m.config.Dialect)
	if err != nil {
		return err
	}

	for _, d := range m.data {
		migFS, err := fs.Sub(d.FS, d.Path)
		if err != nil {
			return fmt.Errorf("failed to create sub-FS for path %q: %w", d.Path, err)
		}
		provider, err := goose.NewProvider(dialect, db, migFS,
			goose.WithTableName(fmt.Sprintf("goose_migration_%s", d.Name)),
		)
		if err != nil {
			return fmt.Errorf("failed to create goose provider for %q: %w", d.Name, err)
		}
		migStart := time.Now()
		if _, err = provider.Up(ctx); err != nil {
			return fmt.Errorf("migration %q failed: %w", d.Name, err)
		}
		log.Info().Str("component", "gorm").Str("dialect", m.config.Dialect).Str("database", m.config.Name).Str("name", d.Name).Str("duration", time.Since(migStart).String()).Msg("migration set complete")
	}

	return nil

}

// MigratorParams defines the dependencies required to initialize a database migrator, including configuration and migration data.
type MigratorParams struct {
	fx.In
	Config *MigratorGormConfig
	Data   []MigratorData
}

// NewMigrator creates a new instance of migrate.Migrator using the provided MigratorParams configuration.
func NewMigrator(params MigratorParams) migrate.Migrator {
	return &migrator{
		config: &params.Config.GormConfig,
		data:   params.Data,
	}
}
