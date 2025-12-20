package gorm

import (
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
)

// Index collects constructors for implementations in an fx module
func Index() fx.Option {
	return fx.Module("example.data.gorm", fx.Provide(gorm.NewDB, gorm.NewContextBuilder, NewCategoryRepository, NewProductRepository, NewThemeRepository))
}
