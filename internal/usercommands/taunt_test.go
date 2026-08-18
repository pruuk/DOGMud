package usercommands

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// This structural boundary test catches a wrapper that restores hardcoded
// partial/full branches or renders the structured defy outcome more than once.
func TestTauntRoutesStructuredDefyOutcomeExactlyOnce(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(thisFile), "taunt.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var renderCalls, legacyBranches int
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			if ident, ok := n.Fun.(*ast.Ident); ok && ident.Name == "sendChannelDefenceMessages" {
				renderCalls++
			}
		case *ast.SelectorExpr:
			if n.Sel.Name == "Defied" || n.Sel.Name == "FullyDefied" {
				legacyBranches++
			}
		}
		return true
	})
	if renderCalls != 1 {
		t.Fatalf("Taunt sendChannelDefenceMessages calls = %d, want exactly 1", renderCalls)
	}
	if legacyBranches != 0 {
		t.Fatalf("Taunt retains %d legacy Defied/FullyDefied branches", legacyBranches)
	}
}
