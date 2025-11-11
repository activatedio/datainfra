package data

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
)

// FilterByLabels filters a slice of elements by matching their labels against a given selector.
// Returns the filtered slice or an error if the elements do not implement the WithLabels interface.
func FilterByLabels[E any](sel labels.Selector, in []E) ([]E, error) {
	var result []E
	for _, e := range in {

		var a any = e

		if wl, ok := a.(WithLabels); ok {
			if sel.Matches(labels.Set(wl.GetLabels())) {
				result = append(result, e)
			}
		} else {
			return nil, errors.New("type does not have labels to select")
		}

	}
	return result, nil
}
