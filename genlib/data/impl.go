package data

import (
	"fmt"
	"reflect"
)

type implementationGetterOptions struct {
	filter func(any) bool
}

// ImplementationOption is a functional option for GetImplementation and GetImplementations
type ImplementationOption[T any] func(*implementationGetterOptions)

// WithTest keeps only the implementations for which the predicate returns
// true. It is how a caller picks one of several implementations of the same
// type on one entry — a type carrying two Associate markers, say, selects the
// one for the child it is emitting with
// WithTest[Associate](func(in Associate) bool { return in.ChildType == child }).
//
// The predicate takes the implementation by value and must RETURN its verdict.
// Before v0.22.0 it took a func(T) with no return, was invoked for its side
// effect on a copy, and the filter then kept every value of the right type —
// so selection silently fell back to type alone and a second implementation
// of one type panicked GetImplementation.
func WithTest[T any](t func(in T) bool) ImplementationOption[T] {
	return func(opts *implementationGetterOptions) {
		opts.filter = func(in any) bool {
			tmp, ok := in.(T)
			return ok && t(tmp)
		}
	}
}

// GetImplementation returns the implementation of a data type, panicing if more than one is found
func GetImplementation[I any](e *Entry, opts ...ImplementationOption[I]) *I {

	res := GetImplementations[I](e, opts...)

	switch {
	case len(res) == 0:
		return nil
	case len(res) > 1:
		panic(fmt.Sprintf("%d implementations of %s found on entry %s; pass WithTest to select one",
			len(res), reflect.TypeFor[I](), e.Type))
	default:
		return &res[0]
	}
}

// GetImplementations returns implementations of a given data type
func GetImplementations[I any](e *Entry, opts ...ImplementationOption[I]) []I {

	o := &implementationGetterOptions{
		filter: func(any) bool { return true },
	}

	for _, opt := range opts {
		opt(o)
	}

	var results []I

	for _, i := range e.Implementations {
		if tmp, ok := i.(I); ok {
			if o.filter(tmp) {
				results = append(results, tmp)
			}
		}
	}
	return results
}

// HasImplementation returns true if the entry has an implementation of the given type
func HasImplementation[I any](e *Entry) bool {
	return len(GetImplementations[I](e)) > 0
}
