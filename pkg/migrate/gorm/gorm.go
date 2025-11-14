package gorm

import (
	"fmt"
	"io/fs"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/migrate"
	"github.com/pressly/goose/v3"
	"go.uber.org/fx"
)

type MigratorData struct {
	Name string
	FS   fs.FS
	Path string
}

type migrator struct {
	config *datagorm.GormConfig
	data   []MigratorData
}

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

type MigratorParams struct {
	fx.In
	Config *MigratorGormConfig
	Data   []MigratorData
}

func NewMigrator(params MigratorParams) migrate.Migrator {
	return &migrator{
		config: &params.Config.GormConfig,
		data:   params.Data,
	}
}
