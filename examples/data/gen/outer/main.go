// Package main generates the example's outer proxy chains. It reads the
// interfaces the first step (../main.go) wrote, so it runs after it.
package main

import (
	"reflect"

	"github.com/activatedio/datainfra/examples/data/repository"
	"github.com/activatedio/datainfra/genlib/data/outer"
)

//go:generate go run .

func main() {
	outer.Generate(outer.Main{
		Dir:        "../../repository/outer",
		Package:    "outer",
		ModuleName: "example.data.outer",
		Inputs: []outer.Input{
			{Interface: reflect.TypeFor[repository.ProductRepository](), Proxies: outer.Audited},
			{Interface: reflect.TypeFor[repository.TagRepository](), Proxies: outer.Standard},
		},
	})
}
