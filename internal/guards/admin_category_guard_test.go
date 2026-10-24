package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Messaging M7 slice 4 (#371): staff output rides messaging.CategoryAdmin.
// The admin set is DERIVED from the command registry, never from file names
// (admin.bounty.go is a player command): the handlers registered admin-only,
// and every function or method in the same package they reach, minus
// anything a player command also reaches. None of those may pass
// messaging.CategorySystem. Staff-only subcommands of player commands
// (party admin, weather's admin subcommands, bounty declare) are outside
// what a registry can see; the admin ledger covers them.

type pkgSrc struct {
	files map[string]*ast.File // rel path -> file
	fset  *token.FileSet
}

func parsePkg(t *testing.T, root, dir string) pkgSrc {
	t.Helper()
	p := pkgSrc{files: map[string]*ast.File{}, fset: token.NewFileSet()}
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, `.go`) || strings.HasSuffix(n, `_test.go`) {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(dir, n))
		f, err := parser.ParseFile(p.fset, rel, mustRead(t, filepath.Join(root, rel)), 0)
		if err != nil {
			t.Fatal(err)
		}
		p.files[rel] = f
	}
	return p
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// handlerName is the function or method a registration names.
func handlerName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	}
	return ``
}

// registrations returns the admin and player handler names a package
// registers: userCommands map entries {Func, _, _, adminOnly}, and
// AddUserCommand(name, fn, _, adminOnly) / RegisterCommand(name, fn, _, _, adminOnly) calls.
func registrations(p pkgSrc) (admin, player map[string]bool) {
	admin, player = map[string]bool{}, map[string]bool{}
	mark := func(fn ast.Expr, flag ast.Expr) {
		name := handlerName(fn)
		if name == `` {
			return
		}
		if id, ok := flag.(*ast.Ident); ok && id.Name == `true` {
			admin[name] = true
		} else {
			player[name] = true
		}
	}
	for _, f := range p.files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.CompositeLit:
				if id, ok := v.Type.(*ast.Ident); ok && id.Name == `CommandAccess` && len(v.Elts) == 4 {
					mark(v.Elts[0], v.Elts[3])
				}
				if mt, ok := v.Type.(*ast.MapType); ok {
					if id, ok := mt.Value.(*ast.Ident); ok && id.Name == `CommandAccess` {
						for _, el := range v.Elts {
							if kv, ok := el.(*ast.KeyValueExpr); ok {
								if cl, ok := kv.Value.(*ast.CompositeLit); ok && len(cl.Elts) == 4 {
									mark(cl.Elts[0], cl.Elts[3])
								}
							}
						}
					}
				}
			case *ast.CallExpr:
				switch calledName(v) {
				case `AddUserCommand`:
					if len(v.Args) == 4 {
						mark(v.Args[1], v.Args[3])
					}
				case `RegisterCommand`:
					if len(v.Args) == 5 {
						mark(v.Args[1], v.Args[4])
					}
				}
			}
			return true
		})
	}
	return admin, player
}

// funcIndex maps a function or method name to its declarations.
func funcIndex(p pkgSrc) map[string][]*ast.FuncDecl {
	idx := map[string][]*ast.FuncDecl{}
	for _, f := range p.files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				idx[fd.Name.Name] = append(idx[fd.Name.Name], fd)
			}
		}
	}
	return idx
}

// reach is every declaration reachable from roots by name (calls, method
// values and function values alike), within the package. A selector's name
// (x.Name) follows only methods and a bare identifier only plain functions,
// so a field such as user.Character.Room, a literal key {Room: id} or a
// local struct field never reaches the Room handler.
func reach(idx map[string][]*ast.FuncDecl, roots map[string]bool) map[*ast.FuncDecl]bool {
	seen := map[*ast.FuncDecl]bool{}
	var queue []*ast.FuncDecl
	for r := range roots {
		queue = append(queue, idx[r]...)
	}
	for len(queue) > 0 {
		fd := queue[0]
		queue = queue[1:]
		if seen[fd] {
			continue
		}
		seen[fd] = true
		sels := map[*ast.Ident]bool{}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.Field:
				for _, nm := range v.Names {
					sels[nm] = true // a local struct field or a parameter
				}
			case *ast.CompositeLit:
				for _, el := range v.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if k, ok := kv.Key.(*ast.Ident); ok {
							sels[k] = true // a field key such as {Room: id}
						}
					}
				}
			case *ast.SelectorExpr:
				sels[v.Sel] = true
				for _, d := range idx[v.Sel.Name] {
					if d.Recv != nil {
						queue = append(queue, d)
					}
				}
			case *ast.Ident:
				if sels[v] {
					return true
				}
				for _, d := range idx[v.Name] {
					if d.Recv == nil {
						queue = append(queue, d)
					}
				}
			}
			return true
		})
	}
	return seen
}

func adminSystemHits(p pkgSrc) (hits []string, adminFuncs int) {
	admin, player := registrations(p)
	idx := funcIndex(p)
	staff, open := reach(idx, admin), reach(idx, player)
	for fd := range staff {
		if open[fd] {
			continue
		}
		adminFuncs++
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == `CategorySystem` {
				hits = append(hits, p.fset.Position(sel.Pos()).String()+`: `+fd.Name.Name+` (reached only from admin-only commands) sends on CategorySystem`)
			}
			return true
		})
	}
	sort.Strings(hits)
	return hits, adminFuncs
}

func TestAdminOutputRidesCategoryAdmin(t *testing.T) {
	root := repoRoot(t)
	total := 0
	for _, dir := range []string{`internal/usercommands`, `modules/aicompanion`, `modules/playtest`} {
		hits, n := adminSystemHits(parsePkg(t, root, dir))
		total += n
		for _, h := range hits {
			t.Error(h)
		}
	}
	t.Logf(`%d admin-only functions checked`, total)
	if total < 50 {
		t.Fatalf(`found only %d admin-only functions; the registry reader is broken`, total)
	}
}

func TestAdminGuardCanFail(t *testing.T) {
	src := "package p\n" +
		"var userCommands = map[string]CommandAccess{\n" +
		"  `zap`: {Zap, false, true, true},\n" +
		"  `look`: {Look, false, true, false},\n" +
		"}\n" +
		"func Zap(u U) { zapHelper(u); shared(u) }\n" +
		"func zapHelper(u U) { u.SendText(messaging.CategorySystem, `zapped`) }\n" +
		"func Look(u U) { shared(u); _ = u.Zap; _ = E{Zap: 1}; type d struct{ Zap int } }\n" +
		"func shared(u U) { u.SendText(messaging.CategorySystem, `a report`) }\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, `probe.go`, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	hits, _ := adminSystemHits(pkgSrc{files: map[string]*ast.File{`probe.go`: f}, fset: fset})
	if len(hits) != 1 || !strings.Contains(hits[0], `probe.go:7`) || !strings.Contains(hits[0], `zapHelper`) {
		t.Fatalf(`want one hit in zapHelper, got %v`, hits)
	}
}
