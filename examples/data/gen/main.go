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
			Type:       reflect.TypeFor[model.Category](),
			Operations: data.OperationsCrud,
		},
		{
			Type:       reflect.TypeFor[model.Product](),
			Operations: data.OperationsCrud,
		},
		{
			Type:       reflect.TypeFor[model.ProductCategory](),
			Operations: data.OperationsCrud,
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
	})

}
