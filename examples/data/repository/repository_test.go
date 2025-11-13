package repository_test

import (
	"os"
	"testing"

	"github.com/activatedio/datainfra/examples/data/repository/gorm"
	gorm2 "github.com/activatedio/datainfra/pkg/data/gorm"
	gormtesting "github.com/activatedio/datainfra/pkg/data/gorm/testing"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"go.uber.org/fx"
)

var (
	AppFixtures []datatesting.AppFixture
)

func NewStaticGormConfig(in *gorm2.GormConfig) func() *gorm2.GormConfig {
	return func() *gorm2.GormConfig {
		return in
	}
}

func TestMain(m *testing.M) {

	AppFixtures = []datatesting.AppFixture{
		gormtesting.NewAppFixture("gorm1", fx.Module("testing", gorm.Index(), fx.Provide(NewStaticGormConfig(&gorm2.GormConfig{
			Dialect:                  "sqlite",
			EnableDefaultTransaction: true,
			EnableSQLLogging:         true,
			Name:                     "foobar",
		})))),
		gormtesting.NewAppFixture("gorm2", fx.Module("testing", gorm.Index(), fx.Provide(NewStaticGormConfig(&gorm2.GormConfig{
			Dialect:                  "sqlite",
			EnableDefaultTransaction: true,
			EnableSQLLogging:         true,
			Name:                     "foobar2",
		})))),
	}

	for _, fixture := range AppFixtures {
		fixture.BeforeAll()
	}

	rc := m.Run()

	for _, fixture := range AppFixtures {
		fixture.AfterAll()
	}

	os.Exit(rc)

}
