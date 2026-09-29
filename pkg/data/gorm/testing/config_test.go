package testing

import (
	"testing"

	gorm2 "github.com/activatedio/datainfra/pkg/data/gorm"
)

func TestStaticConfigMigratesInTheAppsSchema(t *testing.T) {
	owner := &gorm2.Config{Dialect: "postgres", Host: "h", Port: 1, Username: "o", Password: "p", Name: "postgres", SSLMode: "require"}
	app := &gorm2.Config{Dialect: "postgres", Host: "h", Port: 1, Username: "o", Password: "p", Name: "app", Schema: "fleet"}
	got := NewStaticGormTestingConfig(owner, app)().MigratorGormConfig.GormConfig
	if got.Schema != "fleet" || got.Name != "app" || got.SSLMode != "require" {
		t.Errorf("migrator config = %+v", got)
	}
}
