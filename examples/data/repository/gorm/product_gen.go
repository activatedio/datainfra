package gorm

import (
	"context"

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
	data.SearchTemplate[*model.Product]
	categoryRepository repository.CategoryRepository
}

// ProductRepositoryParams are the parameters for ProductRepository
type ProductRepositoryParams struct {
	fx.In
	CategoryRepository repository.CategoryRepository
}

// NewProductRepository creates a new ProductRepository
func NewProductRepository(params ProductRepositoryParams) repository.ProductRepository {
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
	return &productRepositoryImpl{
		Template: template,
		CrudTemplate: gorm.NewMappingCrudTemplate[*model.Product, *ProductInternal, string](gorm.MappingCrudTemplateImplOptions[*model.Product, *ProductInternal, string]{
			Template:    template,
			FindBuilder: gorm.SingleFindBuilder[string]("products.SKU"),
		}),
		SearchTemplate: gorm.NewMappingSearchTemplate[*model.Product, *ProductInternal](gorm.MappingSearchTemplateParams[*model.Product, *ProductInternal]{
			Template: template,
		}),
		categoryRepository: params.CategoryRepository,
	}
}

func (r *productRepositoryImpl) AssociateCategories(ctx context.Context, key string, add []string, remove []string) error {
	return gorm.Associate[string, string](ctx, gorm.AssociateParams[string, string]{
		AssociationTable: "product_categories",
		ParentColumnName: "product_id",
		ChildColumnName:  "category_id",
		ParentKey:        key,
		Add:              add,
		Remove:           remove,
		ParentRepository: r,
		ChildRepository:  r.categoryRepository,
	})
}
