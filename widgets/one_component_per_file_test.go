package widgets

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOneComponentPerFile pins the layout basic.go, data.go and input.go used
// to violate: five widget structs (rule, progressBar, spinner, sparkline,
// badge) sat in one file, and a reader had to scroll past four unrelated
// components to find the one they came for. Parses every source file with
// go/parser -- the actual Go parser, not a text scan -- and counts its
// top-level struct declarations.
//
// A file may declare zero structs (a registry file, a shared helper) or
// exactly one (a component). More than one fails, by name, so the fix is
// obvious from the failure alone.
func TestOneComponentPerFile(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		require.NoError(t, err, "parsing %s", name)

		var structs []string
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := ts.Type.(*ast.StructType); ok {
					structs = append(structs, ts.Name.Name)
				}
			}
		}

		assert.LessOrEqual(t, len(structs), 1,
			"%s defines %d component structs (%v); split each into its own file", name, len(structs), structs)
	}
}
