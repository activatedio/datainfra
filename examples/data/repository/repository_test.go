package repository_test

import (
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	"go.uber.org/fx"

	"github.com/activatedio/datainfra/examples/data/repository/gorm"
	gormmigrations "github.com/activatedio/datainfra/examples/data/repository/gorm/migrations"
	gormtestdata "github.com/activatedio/datainfra/examples/data/repository/testdata/gorm"
	fs2 "github.com/activatedio/datainfra/pkg/data/fs"
	gorm2 "github.com/activatedio/datainfra/pkg/data/gorm"
	gormtesting "github.com/activatedio/datainfra/pkg/data/gorm/testing"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/activatedio/datainfra/pkg/migrate"
	gormmigrate "github.com/activatedio/datainfra/pkg/migrate/gorm"
)

var (
	AppFixtures []datatesting.AppFixture
)

type ProfileMetadata struct {
	Name string
}

type MigrationData struct {
	Dialect string
}

func TestMain(m *testing.M) {

	const (
		DialectPostgres = "postgres"
		DialectSqlite   = "sqlite"
	)

	dbTemp, err := os.CreateTemp("", "unit")
	name := fmt.Sprintf("unit_%d_%d", time.Now().UnixNano(), os.Getpid())

	if err != nil {
		panic(err)
	}

	defer func() {
		if err = os.Remove(dbTemp.Name()); err != nil {
			panic(err)
		}
	}()

	makeFS := func(data *MigrationData, base fs.FS) fs.FS {
		fs, _err := fs2.TemplateFS(fs2.WithSource(base), fs2.WithData(data))
		if _err != nil {
			panic(_err)
		}
		return fs
	}

	makeMigrations := func(dialect string) []gormmigrate.MigratorData {
		return []gormmigrate.MigratorData{
			{
				Name: "main",
				FS: makeFS(&MigrationData{
					Dialect: dialect,
				}, gormmigrations.Files),
				Path: ".",
			},
			{
				Name: "test",
				FS: makeFS(&MigrationData{
					Dialect: dialect,
				}, gormtestdata.Files),
				Path: ".",
			},
		}
	}

	// makeLayers turns the two goose sets into the fixture's stack: the
	// schema, then the seed rows. Both sets carry exact Down sections.
	makeLayers := func(dialect string) any {
		return func(cfg *gormmigrate.MigratorGormConfig) ([]migrate.Layer, error) {
			var layers []migrate.Layer
			for _, d := range makeMigrations(dialect) {
				l, err := gormmigrate.NewGooseLayer(cfg, d)
				if err != nil {
					return nil, err
				}
				layers = append(layers, l)
			}
			return layers, nil
		}
	}

	postgresHost := os.Getenv("POSTGRES_HOST")

	if postgresHost == "" {
		postgresHost = "127.0.0.1"
	}

	AppFixtures = []datatesting.AppFixture{
		datatesting.Bind(gormtesting.NewAppFixture(DialectSqlite, fx.Module("testing", gorm.Index(),
			fx.Provide(func() *ProfileMetadata {
				return &ProfileMetadata{
					Name: DialectSqlite,
				}
			}, gormtesting.NewStaticGormTestingConfig(&gorm2.Config{
				Dialect:                  DialectSqlite,
				EnableDefaultTransaction: true,
				EnableSQLLogging:         true,
				Name:                     dbTemp.Name(),
			}, &gorm2.Config{
				Dialect:                  DialectSqlite,
				EnableDefaultTransaction: true,
				EnableSQLLogging:         true,
				Name:                     dbTemp.Name(),
			}), makeLayers(DialectSqlite)))), datatesting.Requirement{}),
		datatesting.Bind(gormtesting.NewAppFixture(DialectPostgres, fx.Module("testing", gorm.Index(),
			fx.Provide(func() *ProfileMetadata {
				return &ProfileMetadata{
					Name: DialectPostgres,
				}
			}, gormtesting.NewStaticGormTestingConfig(&gorm2.Config{
				Dialect:                  DialectPostgres,
				Host:                     postgresHost,
				Port:                     5432,
				Username:                 "postgres",
				Password:                 "supersecret",
				EnableDefaultTransaction: true,
				EnableSQLLogging:         true,
				Name:                     "postgres",
			}, &gorm2.Config{
				Dialect:                  "postgres",
				Host:                     postgresHost,
				Port:                     5432,
				EnableDefaultTransaction: true,
				EnableSQLLogging:         true,
				Name:                     name,
				Username:                 name,
				Password:                 name,
			}), makeLayers(DialectPostgres)))), datatesting.Requirement{}),
	}

	rc := m.Run()

	for _, fixture := range AppFixtures {
		if err = fixture.Cleanup(); err != nil {
			panic(err)
		}
	}

	os.Exit(rc)

}
