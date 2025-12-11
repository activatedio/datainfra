package simple1

import (
	"embed"

	"github.com/activatedio/datainfra/pkg/symbols"
)

//go:embed *.txt
var Files embed.FS

func SymbolSources() symbols.Symbols {
	return map[string]any{
		"key1": "value1",
		"key2": "value2",
	}
}
