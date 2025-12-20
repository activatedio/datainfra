package model

// Category represents a categorization entity with a unique name and description.
type Category struct {
	Name        string `data:"key" gorm:"primaryKey"`
	Description string
}

// GetKey returns the name of the Category instance.
func (c *Category) GetKey() string {
	return c.Name
}

// Product represents an item with a Stock Keeping Unit (SKU) and a description.
type Product struct {
	SKU         string `data:"key" gorm:"primaryKey"`
	Description string
}

// GetStringID returns the SKU value of the Product instance.
func (p *Product) GetStringID() string {
	return p.SKU
}

// Theme represents a thematic entity with a unique name and description.
type Theme struct {
	Name        string `data:"key" gorm:"primaryKey"`
	Description string
}

// GetStringID returns the SKU value of the Product instance.
func (t *Theme) GetStringID() string {
	return t.Name
}
