package model

type Category struct {
	Name        string `data:"key" gorm:"primaryKey"`
	Description string
}

type Product struct {
	SKU         string `data:"key" gorm:"primaryKey"`
	Description string
}
