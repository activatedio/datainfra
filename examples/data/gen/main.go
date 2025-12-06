// Package main contains the main method for generation
package main

import (
	"reflect"

	"github.com/activatedio/datainfra/examples/data/model"
	"github.com/activatedio/datainfra/genlib/data"
	"github.com/activatedio/datainfra/genlib/data/gorm"
)

//go:generate go run .

func main() {

	ds := []data.Entry{
		{
			Type: reflect.TypeFor[model.Category](),
			Implementations: []any{
				data.Crud{
					Operations: data.OperationsCrud,
				},
				data.FilterKeys{},
			},
		},
		{
			Type: reflect.TypeFor[model.Product](),
			Implementations: []any{
				data.Crud{
					Operations: data.OperationsCrud,
				},
				data.Search{},
				data.Associate{
					ChildType: reflect.TypeFor[model.Category](),
				},
			},
		},
	}

	data.NewDataRegistry().RunFilePathHandler("../repository/types.go", &data.Types{
		Package: "repository",
		Entries: ds,
	})

	gorm.NewDataRegistry().RunDirectoryPathHandler("../repository/gorm", &gorm.DirectoryMain{
		InterfaceImport: "github.com/activatedio/datainfra/examples/data/repository",
		Package:         "gorm",
		Entries:         ds,
		GenerateIndex:   true,
		IndexModule:     "example.data.gorm",
	})

}
