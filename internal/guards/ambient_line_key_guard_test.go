package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// #317 repeat limit (owner 2026-10-31): every ambient line names its
// repeat-limit identity. An ambient.Line literal without a Key field skips
// the limit silently, so a new send site would repeat itself forever. This
// guard finds every ambient.Line composite literal outside internal/ambient
// and requires a Key: field in it.

// ambientLinesWithoutKey reports every ambient.Line literal in src that has
// no Key field, and how many ambient.Line literals it saw.
func ambientLinesWithoutKey(t *testing.T, rel string, src []byte) (bad []string, found int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != `Line` {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != `ambient` {
			return true
		}
		found++
		for _, el := range lit.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == `Key` {
					return true
				}
			}
		}
		bad = append(bad, rel+`:`+strconv.Itoa(fset.Position(lit.Pos()).Line)+`: ambient.Line without Key`)
		return true
	})
	return bad, found
}

func TestAmbientLineKeyGuard(t *testing.T) {
	root := repoRoot(t)
	skip := filepath.Join(root, `internal`, `ambient`) + string(filepath.Separator)
	scanned, found := 0, 0
	for _, top := range []string{`internal`, `modules`} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, `.go`) || strings.HasSuffix(path, `_test.go`) {
				return err
			}
			if strings.HasPrefix(path, skip) {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			bad, n := ambientLinesWithoutKey(t, filepath.ToSlash(rel), src)
			found += n
			for _, b := range bad {
				t.Error(b)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 100 {
		t.Fatalf(`scanned only %d Go files; the walk is broken`, scanned)
	}
	if found < 6 {
		t.Fatalf(`found only %d ambient.Line literals; the finder is broken`, found)
	}
}

// TestAmbientLineKeyGuardCanFail feeds the finder one keyed and one unkeyed
// literal.
func TestAmbientLineKeyGuardCanFail(t *testing.T) {
	src := []byte(`package p

func f() {
	_ = ambient.Line{Room: r, Source: s, Key: k}
	_ = ambient.Line{Room: r, Source: s}
}
`)
	bad, found := ambientLinesWithoutKey(t, `p/p.go`, src)
	if found != 2 {
		t.Fatalf(`found %d literals, want 2`, found)
	}
	if len(bad) != 1 || bad[0] != `p/p.go:5: ambient.Line without Key` {
		t.Fatalf(`want exactly the unkeyed literal on line 5 reported, got %v`, bad)
	}
}
