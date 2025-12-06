package genlib

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/dave/jennifer/jen"
)

// WithPackage is an interface that defines a method for retrieving the package name as a string.
type WithPackage interface {
	GetPackage() string
}

// FileHandler is a function type used to handle operations on a *jen.File with a given Registry and entry object.
type FileHandler func(f *jen.File, r Registry, entry any)

// DirectoryHandler defines a function type for handling a directory path with a registry and an entry object.
type DirectoryHandler func(dirPath string, r Registry, entry any)

// StatementHandler defines a function type that modifies a jen.Statement based on a Registry and an entry of any type.
type StatementHandler = func(s *jen.Statement, r Registry, entry any) *jen.Statement

// HandlerEntries is a container for managing DirectoryHandlers, FileHandlers, and StatementHandlers categorized by type.
// It includes a mutex for safe concurrent modifications to handler mappings.
// DirectoryHandlers maps types to a list of entries for handling directory paths associated with specific entries.
// FileHandlers maps types to a list of entries for handling file paths associated with specific entries.
// StatementHandlers maps types to a list of entries for handling statement transformations with specific entries.
type HandlerEntries struct {
	mu                sync.Mutex
	DirectoryHandlers map[reflect.Type][]entry[DirectoryHandler]
	FileHandlers      map[reflect.Type][]entry[FileHandler]
	StatementHandlers map[reflect.Type][]entry[StatementHandler]
}

// entry is a generic type that pairs a test function with a handler of type H.
// test is a function to evaluate a condition associated with the entry.
// handler is the implementation of the handler logic.
type entry[H any] struct {
	test    func(e any) bool
	handler H
}

// Key defines a structure to associate a reflect.Type with a validation function for dynamic type handling.
type Key struct {
	t    reflect.Type
	test func(e any) bool
}

// NewKey creates a new Key instance with a type derived from the generic type parameter E and a default test function.
func NewKey[E any]() Key {
	return Key{
		t: reflect.TypeFor[E](),
		test: func(_ any) bool {
			return true
		},
	}
}

// NewKeyWithTest creates a new Key for the specified type with a custom test function to validate values of that type.
func NewKeyWithTest[E any](test func(in E) bool) Key {
	return Key{
		t: reflect.TypeFor[E](),
		test: func(e any) bool {
			tmp := e.(E)
			return test(tmp)
		},
	}
}

// NewHandlerEntries initializes a new instance of HandlerEntries with empty handler maps for directories, files, and statements.
func NewHandlerEntries() *HandlerEntries {
	return &HandlerEntries{
		DirectoryHandlers: make(map[reflect.Type][]entry[DirectoryHandler]),
		FileHandlers:      make(map[reflect.Type][]entry[FileHandler]),
		StatementHandlers: make(map[reflect.Type][]entry[StatementHandler]),
	}
}

// AddDirectoryHandler registers a DirectoryHandler for a specific key, associating it with a type and a test function.
// It ensures thread-safe access and updates the map of DirectoryHandlers of the HandlerEntries structure.
// Returns the updated HandlerEntries instance for chaining.
func (h *HandlerEntries) AddDirectoryHandler(key Key, e DirectoryHandler) *HandlerEntries {
	h.mu.Lock()
	defer h.mu.Unlock()
	es := h.DirectoryHandlers[key.t]
	es = append(es, entry[DirectoryHandler]{
		test:    key.test,
		handler: e,
	})
	h.DirectoryHandlers[key.t] = es
	return h
}

// AddFileHandler registers a FileHandler for a specific key. It ensures thread safety using a mutex during the operation.
func (h *HandlerEntries) AddFileHandler(key Key, e FileHandler) *HandlerEntries {
	h.mu.Lock()
	defer h.mu.Unlock()
	es := h.FileHandlers[key.t]
	es = append(es, entry[FileHandler]{
		test:    key.test,
		handler: e,
	})
	h.FileHandlers[key.t] = es
	return h
}

// AddStatementHandler registers a new StatementHandler for a specific key, associating it with a type and optional test logic.
func (h *HandlerEntries) AddStatementHandler(key Key, e StatementHandler) *HandlerEntries {
	h.mu.Lock()
	defer h.mu.Unlock()
	es := h.StatementHandlers[key.t]
	es = append(es, entry[StatementHandler]{
		test:    key.test,
		handler: e,
	})
	h.StatementHandlers[key.t] = es
	return h
}

// registry is a structure that holds mappings for directory, file, and statement handler entries by type.
type registry struct {
	directoryHandlers map[reflect.Type][]entry[DirectoryHandler]
	fileHandlers      map[reflect.Type][]entry[FileHandler]
	statementHandlers map[reflect.Type][]entry[StatementHandler]
}

// NewRegistry creates a new instance of a Registry with initialized handler maps for directories, files, and statements.
func NewRegistry() Registry {
	return &registry{
		directoryHandlers: map[reflect.Type][]entry[DirectoryHandler]{},
		fileHandlers:      map[reflect.Type][]entry[FileHandler]{},
		statementHandlers: map[reflect.Type][]entry[StatementHandler]{},
	}
}

// Registry defines an interface for managing handler entries and executing handlers for files, directories, and statements.
// Clone creates a copy of the internal storage.
// WithHandlerEntries adds a set of handler entries to the registry and returns the updated instance.
// RunFileHandler executes the file handler with the specified file and entry.
// RunFilePathHandler executes the handler for a specific file path with the given entry.
// RunDirectoryPathHandler executes the handler for a specific directory path with the given entry.
// BuildStatement constructs and returns a statement using the provided entry and handler logic.
type Registry interface {
	// Clone creates a copy of the internal storage
	Clone() Registry
	WithHandlerEntries(entries ...*HandlerEntries) Registry
	RunFileHandler(f *jen.File, entry any)
	RunFilePathHandler(path string, entry any)
	RunDirectoryPathHandler(path string, entry any)
	BuildStatement(stmt *jen.Statement, entry any) *jen.Statement
}

// Clone creates and returns a deep copy of the current registry with its internal storage duplicated.
func (r *registry) Clone() Registry {

	dh := map[reflect.Type][]entry[DirectoryHandler]{}
	for k, v := range r.directoryHandlers {
		dh[k] = v
	}
	fh := map[reflect.Type][]entry[FileHandler]{}
	for k, v := range r.fileHandlers {
		fh[k] = v
	}
	sh := map[reflect.Type][]entry[StatementHandler]{}
	for k, v := range r.statementHandlers {
		sh[k] = v
	}

	return &registry{
		directoryHandlers: dh,
		fileHandlers:      fh,
		statementHandlers: sh,
	}
}

// WithHandlerEntries adds multiple HandlerEntries to the registry, organizing and merging their handlers appropriately.
func (r *registry) WithHandlerEntries(entries ...*HandlerEntries) Registry {

	for _, es := range entries {
		if es.DirectoryHandlers != nil {
			for k, v := range es.DirectoryHandlers {
				exist := r.directoryHandlers[k]
				exist = append(exist, v...)
				r.directoryHandlers[k] = exist
			}
		}
		if es.FileHandlers != nil {
			for k, v := range es.FileHandlers {
				exist := r.fileHandlers[k]
				exist = append(exist, v...)
				r.fileHandlers[k] = exist
			}
		}
		if es.StatementHandlers != nil {
			for k, v := range es.StatementHandlers {
				exist := r.statementHandlers[k]
				exist = append(exist, v...)
				r.statementHandlers[k] = exist
			}
		}
	}

	return r
}

// BuildStatement applies registered StatementHandlers to the provided statement based on the type of the given entry.
func (r *registry) BuildStatement(stmt *jen.Statement, entry any) *jen.Statement {
	for _, e := range r.statementHandlers[reflect.TypeOf(entry)] {
		if e.test(entry) {
			stmt = e.handler(stmt, r, entry)
		}
	}
	return stmt
}

// RunFileHandler iterates over registered file handlers, checking conditions and invoking handlers that match the entry.
func (r *registry) RunFileHandler(f *jen.File, entry any) {
	for _, e := range r.fileHandlers[reflect.TypeOf(entry)] {
		if e.test(entry) {
			e.handler(f, r, entry)
		}
	}
}

// RunDirectoryPathHandler ensures the specified path exists as a directory and executes applicable directory handlers for the entry.
func (r *registry) RunDirectoryPathHandler(path string, entry any) {

	f, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}
		Check(os.MkdirAll(path, 0750))
	} else if !f.IsDir() {
		panic(fmt.Sprintf("path %s is not a directory", path))
	}
	for _, e := range r.directoryHandlers[reflect.TypeOf(entry)] {
		if e.test(entry) {
			e.handler(path, r, entry)
		}
	}

}

// RunFilePathHandler verifies and creates necessary directories, ensures the path is a file, and processes the file entry.
func (r *registry) RunFilePathHandler(path string, entry any) {

	dir := filepath.Dir(path)
	_, err := os.Stat(dir)

	if errors.Is(err, os.ErrNotExist) {
		Check(os.MkdirAll(dir, 0750))
	}

	f, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}
	} else if f.IsDir() {
		panic(fmt.Sprintf("path %s is not a file", path))
	}

	var a any //nolint:staticcheck // ignore SA9003
	a = entry

	if wp, ok := a.(WithPackage); ok {
		WithFile(wp.GetPackage(), path, func(f *jen.File) {
			r.RunFileHandler(f, entry)
		})
	} else {
		panic(fmt.Sprintf("entry %s does not have a GetPackage() string method", reflect.TypeOf(entry).String()))
	}

}
