package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// characters.SetProgressionNotifier defaults to nil, which is silent on
// purpose so tests need no wiring. The cost of that default: deleting the
// registration from main.go silences every skill and stat banner in the game
// while every unit test stays green. This guard is the only thing that sees it.
// It parses the AST because a text match was fooled by a commented-out line.
func TestProgressionNotifierRegisteredAtBoot(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go (test must run from the repo root): %v", err)
	}

	const want = "characters.SetProgressionNotifier(hooks.ProgressionNotifyCallback)"
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		funX, ok := fun.X.(*ast.Ident)
		if !ok || funX.Name != "characters" || fun.Sel.Name != "SetProgressionNotifier" {
			return true
		}
		if len(call.Args) != 1 {
			return true
		}
		arg, ok := call.Args[0].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		argX, ok := arg.X.(*ast.Ident)
		if !ok || argX.Name != "hooks" || arg.Sel.Name != "ProgressionNotifyCallback" {
			return true
		}
		found = true
		return false
	})

	if !found {
		t.Errorf("main.go does not register the progression notifier (%s); every progression line would be silent", want)
	}
}
