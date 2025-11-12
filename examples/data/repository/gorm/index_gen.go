package gorm

import fx "go.uber.org/fx"

func Index() fx.Option {
	return fx.Module("example.data.gorm", fx.Provide(NewCategoryRepository, NewProductRepository, NewProductCategoryRepository))
}
