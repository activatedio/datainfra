package repository

import (
	"context"
	model "github.com/activatedio/datainfra/examples/data/model"
	data "github.com/activatedio/datainfra/pkg/data"
)

// CategoryRepository is a repository for the type Category
type CategoryRepository interface {
	FindByKey(context.Context, string) (*model.Category, error)
	ExistsByKey(context.Context, string) (bool, error)
	ListAll(context.Context, data.ListParams) (*data.List[*model.Category], error)
	Create(context.Context, *model.Category) error
	Update(context.Context, *model.Category) error
	Delete(context.Context, string) error
	DeleteEntity(context.Context, *model.Category) error
}

// ProductRepository is a repository for the type Product
type ProductRepository interface {
	FindByKey(context.Context, string) (*model.Product, error)
	ExistsByKey(context.Context, string) (bool, error)
	ListAll(context.Context, data.ListParams) (*data.List[*model.Product], error)
	Create(context.Context, *model.Product) error
	Update(context.Context, *model.Product) error
	Delete(context.Context, string) error
	DeleteEntity(context.Context, *model.Product) error
	// Need to add search methods here
}
type ProductCategoryKey struct {
	// ProductCategoryKey is the key for ProductCategory
	SKU          string
	CategoryName string
}

// ProductCategoryRepository is a repository for the type ProductCategory
type ProductCategoryRepository interface {
	FindByKey(context.Context, ProductCategoryKey) (*model.ProductCategory, error)
	ExistsByKey(context.Context, ProductCategoryKey) (bool, error)
	ListAll(context.Context, data.ListParams) (*data.List[*model.ProductCategory], error)
	Create(context.Context, *model.ProductCategory) error
	Update(context.Context, *model.ProductCategory) error
	Delete(context.Context, ProductCategoryKey) error
	DeleteEntity(context.Context, *model.ProductCategory) error
	// Need to add search methods here
}
