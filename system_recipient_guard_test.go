package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Messaging M7 slice 4 (#371), ruling R4, rule N1: a confirmation reaches only
// the player whose action produced it. Inside a command handler (any function
// reachable from a registered player or admin command in the same package), a
// SendText* on messaging.CategorySystem must go to a parameter of an enclosing
// function. A local looked up from another id, a range variable over other
// players, or a room is somebody else: that line is a notice or narration and
// belongs on its own category. Hooks are outside this rule (an event has no
// single acting player). A hit is converted; never allow-list a file.

func receiverRoot(e ast.Expr) *ast.Ident {
	for {
		switch v := e.(type) {
		case *ast.Ident:
			return v
		case *ast.SelectorExpr:
			e = v.X
		case *ast.IndexExpr:
			e = v.X
		case *ast.StarExpr:
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		default:
			return nil
		}
	}
}

func paramNames(recv *ast.FieldList, ft *ast.FuncType) map[string]bool {
	names := map[string]bool{}
	for _, fl := range []*ast.FieldList{recv, ft.Params} {
		if fl == nil {
			continue
		}
		for _, f := range fl.List {
			for _, n := range f.Names {
				names[n.Name] = true
			}
		}
	}
	return names
}

func systemRecipientHits(p pkgSrc) []string {
	admin, player := registrations(p)
	roots := map[string]bool{}
	for k := range admin {
		roots[k] = true
	}
	for k := range player {
		roots[k] = true
	}
	var hits []string
	for fd := range reach(funcIndex(p), roots) {
		scopes := []map[string]bool{paramNames(fd.Recv, fd.Type)}
		var walk func(n ast.Node) bool
		walk = func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok {
				scopes = append(scopes, paramNames(nil, lit.Type))
				ast.Inspect(lit.Body, walk)
				scopes = scopes[:len(scopes)-1]
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(sel.Sel.Name, `SendText`) {
				return true
			}
			cat, ok := call.Args[0].(*ast.SelectorExpr)
			if !ok || cat.Sel.Name != `CategorySystem` {
				return true
			}
			root := receiverRoot(sel.X)
			isParam := false
			if root != nil {
				for _, s := range scopes {
					if s[root.Name] {
						isParam = true
					}
				}
			}
			if !isParam {
				var b strings.Builder
				printer.Fprint(&b, p.fset, sel.X)
				hits = append(hits, p.fset.Position(call.Pos()).String()+`: `+fd.Name.Name+` sends CategorySystem to `+b.String()+`, not to the player whose command it is`)
			}
			return true
		}
		ast.Inspect(fd.Body, walk)
	}
	sort.Strings(hits)
	return hits
}

func TestSystemReachesOnlyTheActingPlayer(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal(`runtime.Caller failed`)
	}
	root := filepath.Dir(here)
	// Every package that registers a command: internal/usercommands and the
	// modules found by grep -rln "AddUserCommand\|RegisterCommand(" modules.
	// A package that registers none gives no roots and no hits.
	for _, dir := range []string{`internal/usercommands`, `modules/aicompanion`, `modules/playtest`, `modules/auctions`, `modules/follow`, `modules/gmcp`, `modules/leaderboards`, `modules/cleanup`, `modules/time`, `modules/weather`} {
		for _, h := range systemRecipientHits(parsePkg(t, root, dir)) {
			t.Error(h)
		}
	}
}

func TestSystemRecipientGuardCanFail(t *testing.T) {
	src := "package p\n" +
		"var userCommands = map[string]CommandAccess{`party`: {Party, false, true, false}}\n" +
		"func Party(rest string, user *users.UserRecord) {\n" +
		"  user.SendText(messaging.CategorySystem, `You started a party.`)\n" +
		"  for _, u := range members { u.SendText(messaging.CategorySystem, `X joined.`) }\n" +
		"  if t := users.GetByUserId(9); t != nil { t.SendText(messaging.CategorySystem, `hi`) }\n" +
		"  tell(user)\n" +
		"}\n" +
		"func tell(who *users.UserRecord) { who.SendText(messaging.CategorySystem, `ok`) }\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, `probe.go`, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	hits := systemRecipientHits(pkgSrc{files: map[string]*ast.File{`probe.go`: f}, fset: fset})
	if len(hits) != 2 || !strings.Contains(hits[0], `probe.go:5`) || !strings.Contains(hits[1], `probe.go:6`) {
		t.Fatalf(`want hits at lines 5 and 6, got %v`, hits)
	}
}
