package data

import "reflect"

type Implementation struct {
	RegistryBuilder RegistryBuilder
}

func ExtractImplementationFor[T any](impls []any) *T {

	target := reflect.TypeFor[T]()

	for _, i := range impls {

		switch reflect.TypeOf(i) {
		case target:
			tmp := i.(T)
			return &tmp
		}
	}

	return nil
}
