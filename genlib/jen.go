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

type WithPackage interface {
	GetPackage() string
}

type FileHandler func(f *jen.File, r Registry, entry any)

type DirectoryHandler func(dirPath string, r Registry, entry any)

type StatementHandler = func(s *jen.Statement, r Registry, entry any) *jen.Statement

type HandlerEntries struct {
	mu                sync.Mutex
	DirectoryHandlers map[reflect.Type][]entry[DirectoryHandler]
	FileHandlers      map[reflect.Type][]entry[FileHandler]
	StatementHandlers map[reflect.Type][]entry[StatementHandler]
}

type entry[H any] struct {
	test    func(e any) bool
	handler H
}

type Key struct {
	t    reflect.Type
	test func(e any) bool
}

func NewKey[E any]() Key {
	return Key{
		t: reflect.TypeFor[E](),
		test: func(e any) bool {
			return true
		},
	}
}

func NewKeyWithTest[E any](test func(in E) bool) Key {
	return Key{
		t: reflect.TypeFor[E](),
		test: func(e any) bool {
			tmp := e.(E)
			return test(tmp)
		},
	}
}

func NewHandlerEntries() *HandlerEntries {
	return &HandlerEntries{
		DirectoryHandlers: make(map[reflect.Type][]entry[DirectoryHandler]),
		FileHandlers:      make(map[reflect.Type][]entry[FileHandler]),
		StatementHandlers: make(map[reflect.Type][]entry[StatementHandler]),
	}
}

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

type registry struct {
	directoryHandlers map[reflect.Type][]entry[DirectoryHandler]
	fileHandlers      map[reflect.Type][]entry[FileHandler]
	statementHandlers map[reflect.Type][]entry[StatementHandler]
}

func NewRegistry() Registry {
	return &registry{
		directoryHandlers: map[reflect.Type][]entry[DirectoryHandler]{},
		fileHandlers:      map[reflect.Type][]entry[FileHandler]{},
		statementHandlers: map[reflect.Type][]entry[StatementHandler]{},
	}
}

type Registry interface {
	// Clone creates a copy of the internal storage
	Clone() Registry
	WithHandlerEntries(entries ...*HandlerEntries) Registry
	RunFileHandler(f *jen.File, entry any)
	RunFilePathHandler(path string, entry any)
	RunDirectoryPathHandler(path string, entry any)
	BuildStatement(stmt *jen.Statement, entry any) *jen.Statement
}

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

func (r *registry) BuildStatement(stmt *jen.Statement, entry any) *jen.Statement {
	for _, e := range r.statementHandlers[reflect.TypeOf(entry)] {
		if e.test(entry) {
			stmt = e.handler(stmt, r, entry)
		}
	}
	return stmt
}

func (r *registry) RunFileHandler(f *jen.File, entry any) {
	for _, e := range r.fileHandlers[reflect.TypeOf(entry)] {
		if e.test(entry) {
			e.handler(f, r, entry)
		}
	}
}

func (r *registry) RunDirectoryPathHandler(path string, entry any) {

	f, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}
		Check(os.MkdirAll(path, 0755))
	} else if !f.IsDir() {
		panic(fmt.Sprintf("path %s is not a directory", path))
	}
	for _, e := range r.directoryHandlers[reflect.TypeOf(entry)] {
		if e.test(entry) {
			e.handler(path, r, entry)
		}
	}

}
func (r *registry) RunFilePathHandler(path string, entry any) {

	dir := filepath.Dir(path)
	_, err := os.Stat(dir)

	if errors.Is(err, os.ErrNotExist) {
		Check(os.MkdirAll(dir, 0755))
	}

	f, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}
	} else if f.IsDir() {
		panic(fmt.Sprintf("path %s is not a file", path))
	}

	var a any
	a = entry

	if wp, ok := a.(WithPackage); ok {
		WithFile(wp.GetPackage(), path, func(f *jen.File) {
			r.RunFileHandler(f, entry)
		})
	} else {
		panic(fmt.Sprintf("entry %s does not have a GetPackage() string method", reflect.TypeOf(entry).String()))
	}

}
