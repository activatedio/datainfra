package gorm

import (
	model "github.com/activatedio/datainfra/examples/data/model"
	repository "github.com/activatedio/datainfra/examples/data/repository"
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
)

// ProductInternal is the internal representation of Product
type ProductInternal struct{}

// productRepositoryImpl is the implementation of ProductRepository
type productRepositoryImpl struct {
	Template gorm.MappingTemplate[*model.Product, *ProductInternal]
}

// ProductRepositoryParams are the parameters for ProductRepository
type ProductRepositoryParams struct {
	fx.In
}

// NewProductRepository creates a new ProductRepository
func NewProductRepository(_ ProductRepositoryParams) repository.ProductRepository {
	return &productRepositoryImpl{}
}
