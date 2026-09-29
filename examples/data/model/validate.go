package model

import "errors"

// Validate is what the outer chain's validator proxy calls before a write.
func (p *Product) Validate() error {
	if p.SKU == "" {
		return errors.New("product: sku is required")
	}
	return nil
}

// Validate is what the outer chain's validator proxy calls before a write.
func (t *Tag) Validate() error {
	if t.Name == "" {
		return errors.New("tag: name is required")
	}
	return nil
}
