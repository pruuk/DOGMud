package justice

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Player copy carries no em or en dashes (#430: a guard's arrest speech did).
// Comments may; string literals in the package's non-test files may not.
func TestJusticeStringLiteralsHaveNoDashes(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			scanned++
			if strings.ContainsAny(lit.Value, "\u2014\u2013") {
				t.Errorf("%s: string literal carries a dash: %s", fset.Position(lit.Pos()), lit.Value)
			}
			return true
		})
	}
	if scanned < 20 {
		t.Fatalf("scanned only %d string literals; the walk is not reaching the package", scanned)
	}
}
