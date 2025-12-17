package model

// Category represents a categorization entity with a unique name and description.
type Category struct {
	Name        string `data:"key" gorm:"primaryKey"`
	Description string
}

// Product represents an item with a Stock Keeping Unit (SKU) and a description.
type Product struct {
	SKU         string `data:"key" gorm:"primaryKey"`
	Description string
}

// WithStringID returns the SKU value of the Product instance.
func (p *Product) WithStringID() string {
	return p.SKU
}
