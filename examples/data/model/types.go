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

// Tag represents a free-form label that can be attached to a Product. It
// exists so the example carries two Associate edges on one parent — the
// capability WithTest selects between.
type Tag struct {
	Name  string `data:"key" gorm:"primaryKey"`
	Color string
}

// GetKey returns the name of the Tag instance.
func (t *Tag) GetKey() string {
	return t.Name
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

// LocationKey represents a composite key for a Location entity.
type LocationKey struct {
	City  string `gorm:"primaryKey"`
	State string `gorm:"primaryKey"`
}

// Location represents a physical location with a latitude and longitude.
type Location struct {
	Key       LocationKey `data:"key" gorm:"embedded" db:""`
	Latitude  float64
	Longitude float64
}
