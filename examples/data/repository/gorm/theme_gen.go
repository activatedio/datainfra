package gorm

import (
	model "github.com/activatedio/datainfra/examples/data/model"
	repository "github.com/activatedio/datainfra/examples/data/repository"
	data "github.com/activatedio/datainfra/pkg/data"
	gorm "github.com/activatedio/datainfra/pkg/data/gorm"
	fx "go.uber.org/fx"
)

// ThemeInternal is the internal representation of Theme
type ThemeInternal struct {
	*model.Theme
	TenantID string
}

// SetTenantID sets the tenant ID for the ThemeInternal
func (r *ThemeInternal) SetTenantID(id string) {
	r.TenantID = id
}

// themeRepositoryImpl is the implementation of ThemeRepository
type themeRepositoryImpl struct {
	Template gorm.MappingTemplate[*model.Theme, *ThemeInternal]
	data.CrudTemplate[*model.Theme, string]
}

// ThemeRepositoryParams are the parameters for ThemeRepository
type ThemeRepositoryParams struct {
	fx.In
}

// NewThemeRepository creates a new ThemeRepository
func NewThemeRepository(ThemeRepositoryParams) repository.ThemeRepository {
	template := gorm.NewMappingTemplate[*model.Theme, *ThemeInternal](gorm.MappingTemplateParams[*model.Theme, *ThemeInternal]{
		ContextScope: WithTenantScope(),
		Table:        "themes2",
		ToInternal: func(m *model.Theme) *ThemeInternal {
			return &ThemeInternal{
				Theme: m,
			}
		},
		FromInternal: func(m *ThemeInternal) *model.Theme {
			return m.Theme
		},
	})
	return &themeRepositoryImpl{
		Template: template,
		CrudTemplate: gorm.NewMappingCrudTemplate[*model.Theme, *ThemeInternal, string](gorm.MappingCrudTemplateImplOptions[*model.Theme, *ThemeInternal, string]{
			Template:    template,
			FindBuilder: gorm.SingleFindBuilder[string]("themes2.Name"),
		}),
	}
}
