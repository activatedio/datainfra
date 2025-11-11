package genlib

import (
	"os"

	"github.com/dave/jennifer/jen"
)

// Check panics if error is not nil
func Check(err error) {
	if err != nil {
		panic(err)
	}
}

// Closer is an interface that wraps the basic Close method to release resources.
type Closer interface {
	Close() error
}

// CheckClose ensures the provided Closer's Close method is called and panics if an error is returned.
func CheckClose(c Closer) {
	Check(c.Close())
}

// WritableFile creates or overwrites a file at the given path, opening it with read/write permissions, and returns it.
func WritableFile(path string) *os.File {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644) //nolint:gosec // okay to create file from variable
	Check(err)
	return f
}

// WithFile creates a new Go file with the specified package and path, processes it using the provided handler, and writes it.
func WithFile(pkg, path string, handler func(f *jen.File)) {

	f := jen.NewFile(pkg)

	handler(f)

	out := WritableFile(path)
	defer CheckClose(out)

	_, err := out.WriteString(f.GoString())
	if err != nil {

		f.NoFormat = true
		_, err = out.WriteString(f.GoString())
		Check(err)
	}

}
