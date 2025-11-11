package gorm

import (
	model "github.com/activatedio/datainfra/examples/data/model"
	repository "github.com/activatedio/datainfra/examples/data/repository"
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
)

// CategoryInternal is the internal representation of Category
type CategoryInternal struct{}

// categoryRepositoryImpl is the implementation of CategoryRepository
type categoryRepositoryImpl struct {
	Template gorm.MappingTemplate[*model.Category, *CategoryInternal]
}

// CategoryRepositoryParams are the parameters for CategoryRepository
type CategoryRepositoryParams struct {
	fx.In
}

// NewCategoryRepository creates a new CategoryRepository
func NewCategoryRepository(_ CategoryRepositoryParams) repository.CategoryRepository {
	return &categoryRepositoryImpl{}
}
