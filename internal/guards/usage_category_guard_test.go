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

// Messaging M7 slice 4 (#371): usage rides messaging.CategoryUsage. Two
// rules: a call that passes a messaging category and then a literal holding
// "Usage:" uses CategoryUsage; and every SendText inside a function whose
// name ends in "Usage" and that takes a *users.UserRecord uses
// CategoryUsage. A hit is converted, never allow-listed by file.
func usageCategoryHits(t *testing.T, rel string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var hits []string
	at := func(n ast.Node) string { return rel + `:` + strconv.Itoa(fset.Position(n.Pos()).Line) }
	checkCalls := func(root ast.Node, inUsageFunc bool) {
		ast.Inspect(root, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for i, a := range call.Args {
				sel, ok := a.(*ast.SelectorExpr)
				if !ok || !strings.HasPrefix(sel.Sel.Name, `Category`) {
					continue
				}
				if x, ok := sel.X.(*ast.Ident); !ok || x.Name != `messaging` {
					continue
				}
				if sel.Sel.Name == `CategoryUsage` {
					continue
				}
				if inUsageFunc && calledName(call) == `SendText` {
					hits = append(hits, at(call)+`: usage function sends on `+sel.Sel.Name)
					continue
				}
				if i+1 < len(call.Args) {
					if s, ok := leadLiteral(call.Args[i+1]); ok && strings.Contains(openerTag.ReplaceAllString(s, ``), `Usage:`) {
						hits = append(hits, at(call)+`: "Usage:" line on `+sel.Sel.Name)
					}
				}
			}
			return true
		})
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		checkCalls(fd.Body, strings.HasSuffix(fd.Name.Name, `Usage`) && takesUserRecord(fd))
	}
	return hits
}

func takesUserRecord(fd *ast.FuncDecl) bool {
	for _, p := range fd.Type.Params.List {
		if star, ok := p.Type.(*ast.StarExpr); ok {
			if sel, ok := star.X.(*ast.SelectorExpr); ok && sel.Sel.Name == `UserRecord` {
				return true
			}
		}
	}
	return false
}

func TestUsageRidesCategoryUsage(t *testing.T) {
	root := repoRoot(t)
	scanned := 0
	for _, top := range []string{`internal`, `modules`} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, `.go`) || strings.HasSuffix(path, `_test.go`) {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, h := range usageCategoryHits(t, filepath.ToSlash(rel), src) {
				t.Error(h)
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
}

func TestUsageGuardCanFail(t *testing.T) {
	src := "package p\n" +
		"func a(u U) { u.SendText(messaging.CategorySystem, `Usage: kick <name>`) }\n" +
		"func b(u U) { u.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg=\"yellow\">Usage: %s</ansi>`, x)) }\n" +
		"func fooUsage(user *users.UserRecord) { user.SendText(messaging.CategorySystem, txt) }\n" +
		"func c(u U) { u.SendText(messaging.CategoryUsage, `Usage: kick <name>`) }\n" +
		"func barUsage(user *users.UserRecord) { user.SendText(messaging.CategoryUsage, txt) }\n" +
		"func d(u U) { u.SendText(messaging.CategorySystem, `You kick it.`) }\n"
	hits := usageCategoryHits(t, `probe.go`, []byte(src))
	want := []string{`probe.go:2:`, `probe.go:3:`, `probe.go:4:`}
	if len(hits) != len(want) {
		t.Fatalf(`want %d hits, got %v`, len(want), hits)
	}
	for i, w := range want {
		if !strings.HasPrefix(hits[i], w) {
			t.Fatalf(`hit %d: want %s, got %s`, i, w, hits[i])
		}
	}
}
