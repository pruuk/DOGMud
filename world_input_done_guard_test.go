package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Plan 6 guard: InputDone pops after a command's text and menu events only if
// nothing else queues at or above events.InputDonePriority. The scan reads
// the source as Go (go/parser), so multi-line calls, const blocks and typed
// consts are all seen.

// intLit returns the value of an integer literal, or a unary minus around one.
func intLit(e ast.Expr) (int, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.INT {
			n, err := strconv.Atoi(v.Value)
			return n, err == nil
		}
	case *ast.UnaryExpr:
		if v.Op == token.SUB {
			if n, ok := intLit(v.X); ok {
				return -n, true
			}
		}
	case *ast.ParenExpr:
		return intLit(v.X)
	}
	return 0, false
}

// calleeName is the name a call is made through: Foo or x.Foo.
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// litTypeName is the type name of a composite literal (T{} or pkg.T{}), also
// through a leading &.
func litTypeName(e ast.Expr) string {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	if cl, ok := e.(*ast.CompositeLit); ok {
		return calleeName(cl.Type)
	}
	return ""
}

// queueOverreaches parses every source and returns "file:line: ..." for each
// AddToQueue (priority is arg 2) or sendTextAt (arg 3) call whose priority is
// at or above limit, or cannot be resolved to an integer. Consts are collected
// from every const declaration (blocks and typed consts included) and keyed by
// name alone; a name defined with different values in different packages
// resolves to the largest (conservative). Calls whose first argument is a
// RedrawPrompt or InputDone literal are exempt: they are meant to pop at or
// after limit. seen counts the calls that pass a priority (exempt ones too).
func queueOverreaches(sources map[string]string, limit int) (bad []string, seen int) {
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	consts := map[string]int{}
	for name, src := range sources {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: does not parse: %v", name, err))
			continue
		}
		files[name] = f
		ast.Inspect(f, func(n ast.Node) bool {
			gd, ok := n.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				return true
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, id := range vs.Names {
					if i >= len(vs.Values) {
						break
					}
					if v, ok := intLit(vs.Values[i]); ok {
						if old, had := consts[id.Name]; !had || v > old {
							consts[id.Name] = v
						}
					}
				}
			}
			return true
		})
	}
	for name, f := range files {
		for _, decl := range f.Decls {
			// sendTextAt forwards its own priority parameter to AddToQueue;
			// the real priorities are checked at its callers.
			forwarded := map[string]bool{}
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "sendTextAt" {
				for _, fld := range fd.Type.Params.List {
					for _, id := range fld.Names {
						forwarded[id.Name] = true
					}
				}
			}
			bad, seen = scanDecl(fset, name, decl, forwarded, consts, limit, bad, seen)
		}
	}
	sort.Strings(bad)
	return bad, seen
}

// scanDecl checks the priority-passing calls inside one declaration.
func scanDecl(fset *token.FileSet, name string, decl ast.Decl, forwarded map[string]bool,
	consts map[string]int, limit int, bad []string, seen int) ([]string, int) {
	ast.Inspect(decl, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		argAt := 0
		switch calleeName(call.Fun) {
		case "AddToQueue":
			argAt = 1
		case "sendTextAt":
			argAt = 2
		default:
			return true
		}
		if len(call.Args) <= argAt {
			return true // default priority 0
		}
		seen++
		if len(call.Args) > 0 {
			if tn := litTypeName(call.Args[0]); tn == "RedrawPrompt" || tn == "InputDone" {
				return true
			}
		}
		where := fset.Position(call.Pos())
		prio, ok := intLit(call.Args[argAt])
		if !ok {
			id := calleeName(call.Args[argAt])
			if forwarded[id] {
				return true
			}
			prio, ok = consts[id]
			if id == "" || !ok {
				bad = append(bad, fmt.Sprintf("%s:%d: priority is not a literal or a const the scan can resolve", name, where.Line))
				return true
			}
		}
		if prio >= limit {
			bad = append(bad, fmt.Sprintf("%s:%d: priority %d >= %d", name, where.Line, prio, limit))
		}
		return true
	})
	return bad, seen
}

// The guard can fail: it catches a literal at the limit, a multi-line call, a
// const from a block, a typed const and an unresolvable priority, and lets the
// allowed cases through.
func TestQueueOverreaches_CatchesASampleThatMustFail(t *testing.T) {
	sample := map[string]string{"a.go": `package a

const (
	loud = 150
)

const typed int = 120

func f() {
	events.AddToQueue(events.Message{UserId: 1}, 99)
	u.sendTextAt(cat, txt, typed)
	events.AddToQueue(Foo{
		A: 1,
	}, 120)
	events.AddToQueue(Bar{
		A: 1,
	}, pkg.loud)
	events.AddToQueue(events.Message{}, someVar)
	events.AddToQueue(events.Message{UserId: 1}, 50)
	events.AddToQueue(events.Message{UserId: 1})
	events.AddToQueue(events.RedrawPrompt{UserId: 1}, 100)
	events.AddToQueue(events.InputDone{UserId: 1}, 99)
}
`}
	bad, seen := queueOverreaches(sample, 99)
	require.Equal(t, []string{
		"a.go:10: priority 99 >= 99",
		"a.go:11: priority 120 >= 99",
		"a.go:12: priority 120 >= 99",
		"a.go:15: priority 150 >= 99",
		"a.go:18: priority is not a literal or a const the scan can resolve",
	}, bad)
	require.Equal(t, 8, seen)
}

// Nothing may queue at or above InputDonePriority except the prompt redraw, or
// the done marker could pop before a command's last text or menu event.
func TestInputDonePriority_NothingElseQueuesAtOrAbove(t *testing.T) {
	sources := map[string]string{}
	for _, root := range []string{"internal", "modules", "."} {
		_ = util.WalkDirTolerant(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == "vendor" || d.Name() == "_datafiles" || (root == "." && p != ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				b, rerr := os.ReadFile(p)
				if rerr == nil {
					sources[filepath.ToSlash(p)] = string(b)
				}
			}
			return nil
		})
	}
	require.Greater(t, len(sources), 100, "the scan must actually find the source")
	bad, seen := queueOverreaches(sources, events.InputDonePriority)
	t.Logf("scanned %d files, %d calls pass a priority", len(sources), seen)
	require.Greater(t, seen, 10, "a scan that matches nothing proves nothing")
	require.Empty(t, bad, "these queue at or above InputDonePriority:\n%s", strings.Join(bad, "\n"))
}
