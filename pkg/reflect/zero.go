package reflect

import "reflect"

// ZeroInterface - E must be a pointer
func ZeroInterface[E any]() E {
	return reflect.New(reflect.TypeFor[E]().Elem()).Interface().(E)
}

// NilInterface - E must be a pointer
func NilInterface[E any]() E {
	return reflect.Zero(reflect.TypeFor[E]()).Interface().(E)
}
