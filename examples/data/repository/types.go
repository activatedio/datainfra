package repository

import (
	"context"
	model "github.com/activatedio/datainfra/examples/data/model"
	data "github.com/activatedio/datainfra/pkg/data"
)

// CategoryRepository is a repository for the type Category
type CategoryRepository interface {
	Delete(context.Context, string) error
	DeleteEntity(context.Context, *model.Category) error
	FindByKey(context.Context, string) (*model.Category, error)
	ExistsByKey(context.Context, string) (bool, error)
	ListAll(context.Context, data.ListParams) (*data.List[*model.Category], error)
	Create(context.Context, *model.Category) error
	Update(context.Context, *model.Category) error
}

// ProductRepository is a repository for the type Product
type ProductRepository interface {
	Delete(context.Context, string) error
	DeleteEntity(context.Context, *model.Product) error
	FindByKey(context.Context, string) (*model.Product, error)
	ExistsByKey(context.Context, string) (bool, error)
	ListAll(context.Context, data.ListParams) (*data.List[*model.Product], error)
	Create(context.Context, *model.Product) error
	Update(context.Context, *model.Product) error
	// Need to add search methods here
	// Need to add associate
}
