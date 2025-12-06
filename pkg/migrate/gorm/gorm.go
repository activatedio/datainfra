package gorm

import (
	"fmt"
	"io/fs"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/migrate"
	"github.com/pressly/goose/v3"
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

// Migrate executes database migrations using the configuration and migration data defined in the migrator instance.
func (m *migrator) Migrate() error {

	gdb, err := datagorm.NewDB(m.config)

	if err != nil {
		return err
	}

	db, err := gdb.DB()

	if err != nil {
		return err
	}

	if err = goose.SetDialect(m.config.Dialect); err != nil {
		return err
	}

	for _, d := range m.data {
		goose.SetTableName(fmt.Sprintf("goose_migration_%s", d.Name))
		goose.SetBaseFS(d.FS)
		err = goose.Up(db, d.Path)
		if err != nil {
			return err
		}
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
