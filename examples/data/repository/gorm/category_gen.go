package gorm

import (
	model "github.com/activatedio/datainfra/examples/data/model"
	repository "github.com/activatedio/datainfra/examples/data/repository"
	data "github.com/activatedio/datainfra/pkg/data"
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
)

// CategoryInternal is the internal representation of Category
type CategoryInternal struct {
	*model.Category
}

// categoryRepositoryImpl is the implementation of CategoryRepository
type categoryRepositoryImpl struct {
	Template gorm.MappingTemplate[*model.Category, *CategoryInternal]
	data.CrudTemplate[*model.Category, string]
}

// CategoryRepositoryParams are the parameters for CategoryRepository
type CategoryRepositoryParams struct {
	fx.In
}

// NewCategoryRepository creates a new CategoryRepository
func NewCategoryRepository(_ CategoryRepositoryParams) repository.CategoryRepository {
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
		Template: template, CrudTemplate: gorm.NewMappingCrudTemplate[*model.Category, *CategoryInternal, string](gorm.MappingCrudTemplateImplOptions[*model.Category, *CategoryInternal, string]{
			Template:    template,
			FindBuilder: gorm.SingleFindBuilder[string]("categories.Name"),
		}),
	}
}
