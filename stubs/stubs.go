// Package stubs embeds the code-generation templates used by the `oni make:*`
// generators. Embedding them in the CLI binary means generators work from any
// directory — including a freshly scaffolded project that has no stubs/ folder
// of its own.
package stubs

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed *.stub
var FS embed.FS

// Read returns the contents of the named stub (e.g. "controller").
func Read(kind string) ([]byte, error) {
	b, err := fs.ReadFile(FS, kind+".stub")
	if err != nil {
		return nil, fmt.Errorf("stub %q not found", kind)
	}
	return b, nil
}
