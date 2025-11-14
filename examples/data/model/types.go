package model

type Category struct {
	Name        string `data:"key" gorm:"primaryKey"`
	Description string
}

type Product struct {
	SKU         string `data:"key" gorm:"primaryKey"`
	Description string
}

type ProductCategory struct {
	SKU          string `data:"key" gorm:"primaryKey"`
	CategoryName string `data:"key" gorm:"primaryKey"`
}
