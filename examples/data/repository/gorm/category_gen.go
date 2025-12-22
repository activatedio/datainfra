package gorm

import (
	"context"

	model "github.com/activatedio/datainfra/examples/data/model"
	repository "github.com/activatedio/datainfra/examples/data/repository"
	data "github.com/activatedio/datainfra/pkg/data"
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
	gorm1 "gorm.io/gorm"
)

// CategoryInternal is the internal representation of Category
type CategoryInternal struct {
	*model.Category
}

// categoryRepositoryImpl is the implementation of CategoryRepository
type categoryRepositoryImpl struct {
	Template gorm.MappingTemplate[*model.Category, *CategoryInternal]
	data.CrudTemplate[*model.Category, string]
	data.FilterKeysTemplate[string]
}

// CategoryRepositoryParams are the parameters for CategoryRepository
type CategoryRepositoryParams struct {
	fx.In
}

// NewCategoryRepository creates a new CategoryRepository
func NewCategoryRepository(CategoryRepositoryParams) repository.CategoryRepository {
	template := gorm.NewMappingTemplate[*model.Category, *CategoryInternal](gorm.MappingTemplateParams[*model.Category, *CategoryInternal]{
		Table: "categories",
		ToInternal: func(m *model.Category) *CategoryInternal {
			return &CategoryInternal{
				Category: m,
			}
		},
		FromInternal: func(m *CategoryInternal) *model.Category {
			return m.Category
		},
	})
	return &categoryRepositoryImpl{
		Template: template,
		CrudTemplate: gorm.NewMappingCrudTemplate[*model.Category, *CategoryInternal, string](gorm.MappingCrudTemplateImplOptions[*model.Category, *CategoryInternal, string]{
			Template:    template,
			FindBuilder: gorm.SingleFindBuilder[string]("categories.name"),
		}),
		FilterKeysTemplate: gorm.NewMappingFilterKeysTemplate[*model.Category, *CategoryInternal, string](gorm.MappingFilterKeysTemplateImplOptions[*model.Category, *CategoryInternal, string]{
			Template:   template,
			FindColumn: "name",
		}),
	}
}

func (r *categoryRepositoryImpl) ListByProduct(ctx context.Context, key string, params data.ListParams) (*data.List[*model.Category], error) {
	return r.Template.DoList(ctx, func(tx *gorm1.DB) *gorm1.DB {
		return tx.Joins("INNER JOIN product_categories ON product_categories.category_name = categories.name").Where("product_categories.product_sku=?", key)
	}, params)
}
