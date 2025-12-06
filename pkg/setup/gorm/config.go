package gorm

import datagorm "github.com/activatedio/datainfra/pkg/data/gorm"

// OwnerGormConfig defines the configuration for a GORM data store.
type OwnerGormConfig struct {
	datagorm.Config
}
