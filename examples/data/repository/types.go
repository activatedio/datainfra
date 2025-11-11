package repository

import (
	"context"
	model "github.com/activatedio/datainfra/examples/data/model"
)

// CategoryRepository is a repository for the type Category
type CategoryRepository interface {
	FindByKey(context.Context, string) (*model.Category, error)
}

// ProductRepository is a repository for the type Product
type ProductRepository interface {
	FindByKey(context.Context, string) (*model.Product, error)
}
type ProductCategoryKey struct {
	// ProductCategoryKey is the key for ProductCategory
	SKU          string
	CategoryName string
}

// ProductCategoryRepository is a repository for the type ProductCategory
type ProductCategoryRepository interface {
	FindByKey(context.Context, ProductCategoryKey) (*model.ProductCategory, error)
}
