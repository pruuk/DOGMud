package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNoSkipOnlyPlaceholderTests fails when a test function's entire body is a
// single unconditional t.Skip.
//
// This is the recurrence guard for review finding 9. The position package had
// accumulated 124 test functions of which 87 were pure placeholders, and 40 of
// those cited internal/state/position/control_test.go — a file that does not
// exist, naming 19 tests that exist nowhere in the repo, for functions
// (InitialControlForPair, MarginToDelta) that had themselves been deleted. CI
// reported a large behaviour matrix for a high-risk combat-state subsystem and
// executed half of it. The same pattern had spread to 17 other files; 67 more
// placeholders were removed with it.
//
// A skip-only test is worse than no test: it costs the same to read, it
// inflates the apparent matrix, and its stated justification rots silently
// because nothing ever checks that the coverage it points at still exists.
//
// Conditional skips are untouched and remain legitimate — an integration test
// gated on an env var, a platform-specific case, a fixture that is absent
// locally. Those have a real body; only the degenerate
//
//	func TestFoo(t *testing.T) { t.Skip("covered elsewhere") }
//
// shape is rejected. If behaviour genuinely lives in another test, delete the
// stub and let the other test carry it. If it is not covered, write the test.
func TestNoSkipOnlyPlaceholderTests(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	fset := token.NewFileSet()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			// A syntax error is the compiler's problem to report, not this
			// test's. Skipping it here keeps failures attributable.
			return nil
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			if len(fn.Body.List) != 1 {
				continue
			}
			expr, ok := fn.Body.List[0].(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := expr.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if sel.Sel.Name != "Skip" && sel.Sel.Name != "Skipf" && sel.Sel.Name != "SkipNow" {
				continue
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				rel = path
			}
			offenders = append(offenders, filepath.ToSlash(rel)+":"+
				fset.Position(fn.Pos()).String()[len(fset.Position(fn.Pos()).Filename)+1:]+
				" "+fn.Name.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d test function(s) consist only of an unconditional skip.\n"+
			"Delete the placeholder, or give it a real body. See review finding 9.\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}
