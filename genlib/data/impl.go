package data

func GetImplementation[I any](e *Entry) (*I, bool) {

	for _, i := range e.Implementations {
		if tmp, ok := i.(I); ok {
			return &tmp, true
		}
	}
	return nil, false
}
