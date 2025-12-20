package repository

import (
	"context"

	model "github.com/activatedio/datainfra/examples/data/model"
	data "github.com/activatedio/datainfra/pkg/data"
)

// CategoryRepository is a repository for the type Category
type CategoryRepository interface {
	Update(context.Context, *model.Category) error
	Delete(context.Context, string) error
	DeleteEntity(context.Context, *model.Category) error
	FindByKey(context.Context, string) (*model.Category, error)
	ExistsByKey(context.Context, string) (bool, error)
	ListAll(context.Context, data.ListParams) (*data.List[*model.Category], error)
	Create(context.Context, *model.Category) error
	FilterKeys(ctx context.Context, keys []string) ([]string, error)
	ListByProduct(ctx context.Context, key string, params data.ListParams) (*data.List[*model.Category], error)
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
	Search(ctx context.Context, criteria []*data.SearchPredicate, params *data.PageParams) (*data.List[*data.SearchResult[*model.Product]], error)
	GetSearchPredicates(context.Context) ([]*data.SearchPredicateDescriptor, error)
	AssociateCategories(ctx context.Context, key string, add []string, remove []string) error
	ListByCategory(ctx context.Context, key string, params data.ListParams) (*data.List[*model.Product], error)
}

// ThemeRepository is a repository for the type Theme
type ThemeRepository interface {
	ListAll(context.Context, data.ListParams) (*data.List[*model.Theme], error)
	Create(context.Context, *model.Theme) error
	Update(context.Context, *model.Theme) error
	Delete(context.Context, string) error
	DeleteEntity(context.Context, *model.Theme) error
	FindByKey(context.Context, string) (*model.Theme, error)
	ExistsByKey(context.Context, string) (bool, error)
}
