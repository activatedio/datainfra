package gorm

import (
	model "github.com/activatedio/datainfra/examples/data/model"
	repository "github.com/activatedio/datainfra/examples/data/repository"
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
)

// ProductCategoryInternal is the internal representation of ProductCategory
type ProductCategoryInternal struct{}

// productCategoryRepositoryImpl is the implementation of ProductCategoryRepository
type productCategoryRepositoryImpl struct {
	Template gorm.MappingTemplate[*model.ProductCategory, *ProductCategoryInternal]
}

// ProductCategoryRepositoryParams are the parameters for ProductCategoryRepository
type ProductCategoryRepositoryParams struct {
	fx.In
}

// NewProductCategoryRepository creates a new ProductCategoryRepository
func NewProductCategoryRepository(_ ProductCategoryRepositoryParams) repository.ProductCategoryRepository {
	return &productCategoryRepositoryImpl{}
}
