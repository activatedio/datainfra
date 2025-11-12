package gorm

import (
	model "github.com/activatedio/datainfra/examples/data/model"
	repository "github.com/activatedio/datainfra/examples/data/repository"
	data "github.com/activatedio/datainfra/pkg/data"
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
)

// ProductCategoryInternal is the internal representation of ProductCategory
type ProductCategoryInternal struct {
	*model.ProductCategory
}

// productCategoryRepositoryImpl is the implementation of ProductCategoryRepository
type productCategoryRepositoryImpl struct {
	Template gorm.MappingTemplate[*model.ProductCategory, *ProductCategoryInternal]
	data.CrudTemplate[*model.ProductCategory, repository.ProductCategoryKey]
}

// ProductCategoryRepositoryParams are the parameters for ProductCategoryRepository
type ProductCategoryRepositoryParams struct {
	fx.In
}

// NewProductCategoryRepository creates a new ProductCategoryRepository
func NewProductCategoryRepository(_ ProductCategoryRepositoryParams) repository.ProductCategoryRepository {
	template := gorm.NewMappingTemplate[*model.ProductCategory, *ProductCategoryInternal](gorm.MappingTemplateParams[*model.ProductCategory, *ProductCategoryInternal]{
		Table: "product_categories",
		ToInternal: func(m *model.ProductCategory) *ProductCategoryInternal {
			return &ProductCategoryInternal{
				ProductCategory: m,
			}
		},
		FromInternal: func(m *ProductCategoryInternal) *model.ProductCategory {
			return m.ProductCategory
		},
	})
	return &productCategoryRepositoryImpl{
		Template: template, CrudTemplate: gorm.NewMappingCrudTemplate[*model.ProductCategory, *ProductCategoryInternal, repository.ProductCategoryKey](gorm.MappingCrudTemplateImplOptions[*model.ProductCategory, *ProductCategoryInternal, repository.ProductCategoryKey]{
			Template: template,
		}),
	}
}
