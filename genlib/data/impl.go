package data

type implementationGetterOptions struct {
	filter func(any) bool
}

type ImplementationOption[T any] func(*implementationGetterOptions)

func WithTest[T any](t func(in T)) ImplementationOption[T] {
	return func(opts *implementationGetterOptions) {
		opts.filter = func(in any) bool {
			if tmp, ok := in.(T); ok {
				t(tmp)
				return true
			}
			return false
		}
	}
}

// GetImplementation returns the implementation of a data type, panicing if more than one is found
func GetImplementation[I any](e *Entry, opts ...ImplementationOption[I]) *I {

	res := GetImplementations[I](e, opts...)

	switch {
	case res == nil || len(res) == 0:
		return nil
	case len(res) > 1:
		panic("more than one implementation found")
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
	return GetImplementation[I](e) != nil
}
