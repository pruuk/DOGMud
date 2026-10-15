package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Messaging M7 (#371): the refusal AimBySight returns is a sight refusal, and
// every send of it rides messaging.CategoryRefusal. This guard finds each call
// to AimBySight, takes the identifier its second result is assigned to, and
// requires every SendText(<category>, <that identifier>) in the same function
// to use CategoryRefusal, so a later caller cannot send it on CategorySystem.

func calledName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ``
}

// aimRefusalSends reports every SendText of an AimBySight refusal that is not
// on CategoryRefusal, and how many AimBySight calls it saw.
func aimRefusalSends(t *testing.T, rel string, src []byte) (bad []string, calls int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	check := func(body *ast.BlockStmt) {
		if body == nil {
			return
		}
		idents := map[string]bool{}
		ast.Inspect(body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Rhs) != 1 || len(as.Lhs) != 2 {
				return true
			}
			call, ok := as.Rhs[0].(*ast.CallExpr)
			if !ok || (calledName(call) != `AimBySight` && calledName(call) != `aimAtSight`) {
				return true
			}
			calls++
			if id, ok := as.Lhs[1].(*ast.Ident); ok && id.Name != `_` {
				idents[id.Name] = true
			}
			return true
		})
		if len(idents) == 0 {
			return
		}
		ast.Inspect(body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || calledName(call) != `SendText` || len(call.Args) != 2 {
				return true
			}
			arg, ok := call.Args[1].(*ast.Ident)
			if !ok || !idents[arg.Name] {
				return true
			}
			sel, ok := call.Args[0].(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != `CategoryRefusal` {
				bad = append(bad, rel+`:`+strconv.Itoa(fset.Position(call.Pos()).Line)+`: `+arg.Name+` is not sent on messaging.CategoryRefusal`)
			}
			return true
		})
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			check(fd.Body)
		}
	}
	return bad, calls
}

func TestAimRefusalsRideCategoryRefusal(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal(`runtime.Caller failed`)
	}
	root := filepath.Dir(here)
	scanned, calls := 0, 0
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
			bad, n := aimRefusalSends(t, filepath.ToSlash(rel), src)
			calls += n
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
	if calls < 10 {
		t.Fatalf(`found only %d AimBySight calls; the finder is broken`, calls)
	}
}

// TestAimRefusalGuardCanFail feeds the finder one good and one bad caller.
func TestAimRefusalGuardCanFail(t *testing.T) {
	src := "package p\n" +
		"func good(u U) { name, r := actions.AimBySight(a); if r != `` { u.SendText(messaging.CategoryRefusal, r) }; _ = name }\n" +
		"func bad(u U) { name, r := AimBySight(a); if r != `` { u.SendText(messaging.CategorySystem, r) }; _ = name }\n"
	bad, calls := aimRefusalSends(t, `probe.go`, []byte(src))
	if calls != 2 || len(bad) != 1 || !strings.Contains(bad[0], `probe.go:3`) {
		t.Fatalf(`finder saw %d calls, bad=%v`, calls, bad)
	}
}
