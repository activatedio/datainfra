package model

type Category struct {
	Name        string `data:"key"`
	Description string
}

type Product struct {
	SKU         string `data:"key"`
	Description string
}

type ProductCategory struct {
	SKU          string `data:"key"`
	CategoryName string `data:"key"`
}
