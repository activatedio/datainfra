package gorm

import (
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
)

func Index() fx.Option {
	return fx.Module("example.data.gorm", fx.Provide(gorm.NewDB, gorm.NewContextBuilder, NewCategoryRepository, NewProductRepository, NewProductCategoryRepository))
}
