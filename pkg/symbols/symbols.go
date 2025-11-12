package symbols

import (
	"fmt"
)

// Symbols represents a map where keys are strings and values are of any type, providing utility methods for retrieval.
type Symbols map[string]any

// Source is a function type that returns a Symbols map when invoked.
type Source func() Symbols

// Get retrieves the value associated with the given key from the Symbols map. Returns nil if the key does not exist.
func (s Symbols) Get(key string) any {
	return s[key]
}

// MustGet retrieves the value associated with the given key or panics if the key is not found in the Symbols map.
func (s Symbols) MustGet(key string) any {
	if v, ok := s[key]; ok {
		return v
	}
	panic(fmt.Sprintf("key %s not found", key))
}

// GetString retrieves the value associated with the given key as a string. If the key does not exist, it returns "nil".
func (s Symbols) GetString(key string) string {
	return fmt.Sprintf("%s", s.Get(key))
}

// MustGetString retrieves the value associated with the given key, converts it to a string, and panics if the key is not found.
func (s Symbols) MustGetString(key string) string {
	return fmt.Sprintf("%s", s.MustGet(key))
}
