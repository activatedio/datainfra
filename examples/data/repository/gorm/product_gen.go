package gorm

import (
	model "github.com/activatedio/datainfra/examples/data/model"
	repository "github.com/activatedio/datainfra/examples/data/repository"
	data "github.com/activatedio/datainfra/pkg/data"
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
)

// ProductInternal is the internal representation of Product
type ProductInternal struct {
	*model.Product
}

// productRepositoryImpl is the implementation of ProductRepository
type productRepositoryImpl struct {
	Template gorm.MappingTemplate[*model.Product, *ProductInternal]
	data.CrudTemplate[*model.Product, string]
}

// ProductRepositoryParams are the parameters for ProductRepository
type ProductRepositoryParams struct {
	fx.In
}

// NewProductRepository creates a new ProductRepository
func NewProductRepository(_ ProductRepositoryParams) repository.ProductRepository {
	template := gorm.NewMappingTemplate[*model.Product, *ProductInternal](gorm.MappingTemplateParams[*model.Product, *ProductInternal]{
		Table: "products",
		ToInternal: func(m *model.Product) *ProductInternal {
			return &ProductInternal{
				Product: m,
			}
		},
		FromInternal: func(m *ProductInternal) *model.Product {
			return m.Product
		},
	})
	// implements the SearchHandler interface.
	// implements the SearchHandler interface.
	return &productRepositoryImpl{
		Template: template, CrudTemplate: gorm.NewMappingCrudTemplate[*model.Product, *ProductInternal, string](gorm.MappingCrudTemplateImplOptions[*model.Product, *ProductInternal, string]{
			Template:    template,
			FindBuilder: gorm.SingleFindBuilder[string]("products.SKU"),
		}),
	}
}
