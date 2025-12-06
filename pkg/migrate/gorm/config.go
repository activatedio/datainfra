package gorm

import datagorm "github.com/activatedio/datainfra/pkg/data/gorm"

// MigratorGormConfig defines the configuration for the gorm migrator.
type MigratorGormConfig struct {
	GormConfig datagorm.Config
}
