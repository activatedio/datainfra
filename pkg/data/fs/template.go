package fs

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"text/template"

	"github.com/activatedio/datainfra/pkg/symbols"
)

// TemplateOptions represents the configuration options for template processing.
type TemplateOptions struct {
	source  source
	symbols symbols.Symbols
	data    any
	funcs   func(symbols.Symbols) template.FuncMap
}

// TemplateOption defines a function type for configuring TemplateOptions during template processing.
type TemplateOption func(options *TemplateOptions)

// WithSource sets the source filesystem for template processing in TemplateOptions.
func WithSource(src fs.FS) TemplateOption {
	return func(options *TemplateOptions) {
		options.source = src.(source)
	}
}

// WithSymbolSource configures TemplateOptions to include a set of symbols for template processing.
func WithSymbolSource(symbols symbols.Symbols) TemplateOption {
	return func(options *TemplateOptions) {
		options.symbols = symbols
	}
}

// WithData sets the data for template processing in TemplateOptions.
func WithData(data any) TemplateOption {
	return func(options *TemplateOptions) {
		options.data = data
	}
}

// WithFuncs sets funcs to use for templating
func WithFuncs(funcs func(syms symbols.Symbols) template.FuncMap) TemplateOption {
	return func(options *TemplateOptions) {
		options.funcs = funcs
	}
}

type source interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
}

// TemplateFS creates a virtual filesystem by processing template files and directories based on provided options.
// It allows customization via TemplateOptions to configure the source, symbols, and data for template rendering.
func TemplateFS(opts ...TemplateOption) (fs.FS, error) {
	const (
		defaultRootDir = "."
		tempDirPrefix  = "templatefs"
	)

	// Set up default options with a meaningful initial state
	o := &TemplateOptions{
		data: map[string]any{},
	}
	for _, applyOpt := range opts {
		applyOpt(o)
	}

	// Read the root directory contents from the source
	rootDir := defaultRootDir
	entries, err := o.source.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read root directory '%s': %w", rootDir, err)
	}

	// Create a temporary directory for building the virtual filesystem
	tempDir, err := os.MkdirTemp("", tempDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// Process the directory into the temporary filesystem
	if err := processDir(rootDir, o, entries, tempDir); err != nil {
		return nil, fmt.Errorf("failed to process root directory '%s': %w", rootDir, err)
	}

	// Return the generated virtual filesystem
	return os.DirFS(tempDir), nil
}

func processDir(dir string, o *TemplateOptions, entries []fs.DirEntry, target string) error {

	for _, e := range entries {
		name := path.Join(dir, e.Name())
		targetName := path.Join(target, e.Name())
		if e.IsDir() {
			err := os.Mkdir(targetName, 0750)
			if err != nil {
				return err
			}
			_entries, err := o.source.ReadDir(name)
			if err != nil {
				return err
			}
			err = processDir(name, o, _entries, targetName)
			if err != nil {
				return err
			}
		} else {
			err := processTemplate(name, o, targetName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func processTemplate(name string, o *TemplateOptions, target string) error {

	b, err := o.source.ReadFile(name)

	if err != nil {
		return err
	}

	var t *template.Template
	t = template.New("file")
	if o.funcs != nil {
		t = t.Funcs(o.funcs(o.symbols))
	}
	t, err = t.Parse(string(b))
	if err != nil {
		return err
	}

	var f *os.File
	f, err = os.Create(target) //nolint:gosec // okay to create file from variable
	if err != nil {
		return err
	}

	return t.Execute(f, o.data)
}
