package data

// GetImplementation returns the implementation of a data type
func GetImplementation[I any](e *Entry) *I {

	for _, i := range e.Implementations {
		if tmp, ok := i.(I); ok {
			return &tmp
		}
	}
	return nil
}

func HasImplementation[I any](e *Entry) bool {
	return GetImplementation[I](e) != nil
}
