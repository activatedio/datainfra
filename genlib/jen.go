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
	DirectoryHandlers map[reflect.Type][]DirectoryHandler
	FileHandlers      map[reflect.Type][]FileHandler
	StatementHandlers map[reflect.Type][]StatementHandler
}

func NewHandlerEntries() *HandlerEntries {
	return &HandlerEntries{
		DirectoryHandlers: make(map[reflect.Type][]DirectoryHandler),
		FileHandlers:      make(map[reflect.Type][]FileHandler),
		StatementHandlers: make(map[reflect.Type][]StatementHandler),
	}
}

func (h *HandlerEntries) AddDirectoryHandler(entry any, e DirectoryHandler) *HandlerEntries {
	h.mu.Lock()
	defer h.mu.Unlock()
	es := h.DirectoryHandlers[reflect.TypeOf(entry)]
	es = append(es, e)
	h.DirectoryHandlers[reflect.TypeOf(entry)] = es
	return h
}

func (h *HandlerEntries) AddFileHandler(entry any, e FileHandler) *HandlerEntries {
	h.mu.Lock()
	defer h.mu.Unlock()
	es := h.FileHandlers[reflect.TypeOf(entry)]
	es = append(es, e)
	h.FileHandlers[reflect.TypeOf(entry)] = es
	return h
}

func (h *HandlerEntries) AddStatementHandler(entry any, e StatementHandler) *HandlerEntries {
	h.mu.Lock()
	defer h.mu.Unlock()
	es := h.StatementHandlers[reflect.TypeOf(entry)]
	es = append(es, e)
	h.StatementHandlers[reflect.TypeOf(entry)] = es
	return h
}

type registry struct {
	directoryHandlers map[reflect.Type][]DirectoryHandler
	fileHandlers      map[reflect.Type][]FileHandler
	statementHandlers map[reflect.Type][]StatementHandler
}

func NewRegistry() Registry {
	return &registry{
		directoryHandlers: map[reflect.Type][]DirectoryHandler{},
		fileHandlers:      map[reflect.Type][]FileHandler{},
		statementHandlers: map[reflect.Type][]StatementHandler{},
	}
}

type Registry interface {
	// Clone creates a copy of the internal storage
	Clone() Registry
	WithHandlerEntries(entries ...*HandlerEntries) Registry
	RunFileHandler(f *jen.File, entry any)
	RunFilePathHandler(path string, entry WithPackage)
	RunDirectoryPathHandler(path string, entry any)
	BuildStatement(stmt *jen.Statement, entry any) *jen.Statement
}

func (r *registry) Clone() Registry {

	dh := map[reflect.Type][]DirectoryHandler{}
	for k, v := range r.directoryHandlers {
		dh[k] = v
	}
	fh := map[reflect.Type][]FileHandler{}
	for k, v := range r.fileHandlers {
		fh[k] = v
	}
	sh := map[reflect.Type][]StatementHandler{}
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
	for _, h := range r.statementHandlers[reflect.TypeOf(entry)] {
		stmt = h(stmt, r, entry)
	}
	return stmt
}

func (r *registry) RunFileHandler(f *jen.File, entry any) {
	for _, h := range r.fileHandlers[reflect.TypeOf(entry)] {
		h(f, r, entry)
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
	for _, h := range r.directoryHandlers[reflect.TypeOf(entry)] {
		h(path, r, entry)
	}

}
func (r *registry) RunFilePathHandler(path string, entry WithPackage) {

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

	WithFile(entry.GetPackage(), path, func(f *jen.File) {
		r.RunFileHandler(f, entry)
	})
}
