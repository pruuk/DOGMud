package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// refusableAcquires are the calls that can refuse an item (too much to
// carry) and report it in their result: the acquire seam, and StoreItem
// (Character's and Pet's) beneath it. Character.ReturnToPack is the store
// that never refuses, for an item the character held a moment ago.
var refusableAcquires = map[string]bool{"AcquireItem": true, "AcquireItemForChar": true, "StoreItem": true}

// discardedAcquires lists every production call to a refusable acquire
// whose result is thrown away: a bare call statement, or one assigned only
// to _. src maps a file name to its source; nil reads the repository.
func discardedAcquires(t *testing.T, src map[string]string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var found []string
	visit := func(name string, text any) {
		f, err := parser.ParseFile(fset, name, text, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			var call ast.Expr
			switch s := n.(type) {
			case *ast.ExprStmt:
				call = s.X
			case *ast.AssignStmt:
				for _, l := range s.Lhs {
					if id, ok := l.(*ast.Ident); !ok || id.Name != "_" {
						return true
					}
				}
				if len(s.Rhs) == 1 {
					call = s.Rhs[0]
				}
			default:
				return true
			}
			ce, ok := call.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := ce.Fun.(*ast.SelectorExpr); ok && refusableAcquires[sel.Sel.Name] {
				found = append(found, filepath.ToSlash(name)+":"+strconv.Itoa(fset.Position(ce.Pos()).Line))
			}
			return true
		})
	}
	if src != nil {
		for name, text := range src {
			visit(name, text)
		}
		sort.Strings(found)
		return found
	}
	isSource := func(name string) bool {
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}
	for _, root := range []string{"internal", "modules"} {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && isSource(path) {
				visit(path, nil)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && isSource(e.Name()) {
			visit(e.Name(), nil)
		}
	}
	sort.Strings(found)
	return found
}

// TestNoAcquisitionIgnoresARefusal: an item that was made, bought, foraged,
// salvaged, earned or given must never vanish because its receiver could not
// carry it (#696: a crafted dagger vanished after its materials were spent).
// Every call to a refusable acquire uses its result; a route with nothing to
// roll back uses acquire.AcquireOrDrop, which drops the item at the
// receiver's feet.
func TestNoAcquisitionIgnoresARefusal(t *testing.T) {
	if got := discardedAcquires(t, nil); len(got) > 0 {
		t.Fatalf("an acquire result is ignored, so an item too heavy to carry vanishes (use acquire.AcquireOrDrop, or handle the refusal):\n  %s",
			strings.Join(got, "\n  "))
	}
}

// The guard can fail: it sees a bare call and a call assigned to _, and
// passes a call whose result is used.
func TestNoAcquisitionIgnoresARefusal_CanFail(t *testing.T) {
	src := map[string]string{
		"bare.go":    "package p\nfunc f() { acquire.AcquireItem(u, i, r) }\n",
		"blank.go":   "package p\nfunc f() { _ = acquire.AcquireItemForChar(c, 1, 0, i, r) }\n",
		"checked.go": "package p\nfunc f() { if !acquire.AcquireItem(u, i, r) { g() } }\n",
		"dropped.go": "package p\nfunc f() { acquire.AcquireOrDrop(c, 1, 0, i, r) }\n",
		"store.go":   "package p\nfunc f() { c.StoreItem(i) }\n",
		"back.go":    "package p\nfunc f() { c.ReturnToPack(i) }\n",
	}
	got := discardedAcquires(t, src)
	want := []string{"bare.go:2", "blank.go:2", "store.go:2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}
