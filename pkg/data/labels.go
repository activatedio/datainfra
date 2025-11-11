package data

import (
	"k8s.io/apimachinery/pkg/labels"
)

// Labels is a type alias for labels.Set used to represent a set of label key-value pairs.
type Labels labels.Set

// EmptyLabelsSet returns an empty Labels set
func EmptyLabelsSet() Labels {
	return Labels{}
}

// WithLabels represents an interface for objects that can return a Labels type
type WithLabels interface {
	// GetLabels returns the set of label key-value pairs associated with the implementing object.
	GetLabels() Labels
}
