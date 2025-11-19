package repository_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/activatedio/datainfra/examples/data/repository/gorm"
	gormmigrations "github.com/activatedio/datainfra/examples/data/repository/gorm/migrations"
	gormtestdata "github.com/activatedio/datainfra/examples/data/repository/testdata/gorm"
	gorm2 "github.com/activatedio/datainfra/pkg/data/gorm"
	gormtesting "github.com/activatedio/datainfra/pkg/data/gorm/testing"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	gormmigrate "github.com/activatedio/datainfra/pkg/migrate/gorm"
	"go.uber.org/fx"
)

var (
	AppFixtures []datatesting.AppFixture
)

func TestMain(m *testing.M) {

	dbTemp, err := os.CreateTemp("", "unit")
	name := fmt.Sprintf("unit_%d", time.Now().UnixNano())

	if err != nil {
		panic(err)
	}

	defer func() {
		if err = os.Remove(dbTemp.Name()); err != nil {
			panic(err)
		}
	}()

	migrations := []gormmigrate.MigratorData{
		{
			Name: "main",
			FS:   gormmigrations.Files,
			Path: ".",
		},
		{
			Name: "test",
			FS:   gormtestdata.Files,
			Path: ".",
		},
	}

	AppFixtures = []datatesting.AppFixture{
		gormtesting.NewAppFixture("sqlite", fx.Module("testing", gorm.Index(), fx.Provide(gormtesting.NewStaticGormTestingConfig(&gorm2.GormConfig{
			Dialect:                  "sqlite",
			EnableDefaultTransaction: true,
			EnableSQLLogging:         true,
			Name:                     dbTemp.Name(),
		}, &gorm2.GormConfig{
			Dialect:                  "sqlite",
			EnableDefaultTransaction: true,
			EnableSQLLogging:         true,
			Name:                     dbTemp.Name(),
		}, migrations)))),
		gormtesting.NewAppFixture("postgres", fx.Module("testing", gorm.Index(), fx.Provide(gormtesting.NewStaticGormTestingConfig(&gorm2.GormConfig{
			Dialect:                  "postgres",
			Host:                     "127.0.0.1",
			Port:                     5432,
			Username:                 "postgres",
			Password:                 "supersecret",
			EnableDefaultTransaction: true,
			EnableSQLLogging:         true,
			Name:                     "postgres",
		}, &gorm2.GormConfig{
			Dialect:                  "postgres",
			Host:                     "127.0.0.1",
			Port:                     5432,
			EnableDefaultTransaction: true,
			EnableSQLLogging:         true,
			Name:                     name,
			Username:                 name,
			Password:                 name,
		}, migrations)))),
	}

	rc := m.Run()

	for _, fixture := range AppFixtures {
		if err = fixture.Cleanup(); err != nil {
			panic(err)
		}
	}

	os.Exit(rc)

}
