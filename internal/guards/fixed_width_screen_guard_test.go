package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// fixedWidthTemplates are templates that lay text out in boxes or aligned
// columns at a fixed width. The pipeline folds a line wider than the reader on
// every category it wraps, which collapses the padding of such a row, so a
// sender sends them on CategoryScreen, which is never wrapped. They get narrow
// forms later. Prose templates are absent on purpose: they keep wrapping.
var fixedWidthTemplates = map[string]bool{
	"tables/generic": true, "tables/shoplist": true, "tables/lockpicking": true,
	"descriptions/track": true, "descriptions/identify": true,
	"descriptions/biome": true, "character/conditions": true,
	"character/cooldowns": true, "character/storage": true,
	"character/title": true, "character/status": true,
}

// fixedWidthProducers are helpers that return the text of a fixed-width
// template, so a send that passes their result inline is held to the same rule.
var fixedWidthProducers = map[string]bool{"GetLockRender": true, "getAltTable": true}

var fixedWidthWrappedCats = map[string]bool{
	"CategorySystem": true, "CategoryWarning": true, "CategoryError": true,
	"CategoryRefusal": true, "CategoryUsage": true, "CategoryGroup": true,
	"CategoryBroadcast": true,
}

// fixedWidthFindings is every send in f of fixed-width text on a wrapped
// category. A text is fixed-width when it was assigned from templates.Process
// of a template in fixedWidthTemplates inside the same function, or is the
// result of a fixedWidthProducers call.
func fixedWidthFindings(fset *token.FileSet, f *ast.File) []string {
	var found []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		fixed := map[string]string{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Rhs) != 1 || len(as.Lhs) == 0 {
				return true
			}
			call, ok := as.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && fixedWidthProducers[id.Name] {
				if lhs, ok := as.Lhs[0].(*ast.Ident); ok {
					fixed[lhs.Name] = id.Name
				}
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Process" || len(call.Args) == 0 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			name, err := strconv.Unquote(lit.Value)
			if err != nil || !fixedWidthTemplates[name] {
				return true
			}
			if id, ok := as.Lhs[0].(*ast.Ident); ok {
				fixed[id.Name] = name
			}
			return true
		})
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			send, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || send.Sel.Name != "SendText" {
				return true
			}
			cat, ok := call.Args[0].(*ast.SelectorExpr)
			if !ok || !fixedWidthWrappedCats[cat.Sel.Name] {
				return true
			}
			for _, arg := range call.Args[1:] {
				what := ""
				switch a := arg.(type) {
				case *ast.Ident:
					what = fixed[a.Name]
				case *ast.CallExpr:
					if id, ok := a.Fun.(*ast.Ident); ok && fixedWidthProducers[id.Name] {
						what = id.Name
					}
				}
				if what != "" {
					pos := fset.Position(call.Pos())
					found = append(found, pos.String()+" "+cat.Sel.Name+" carries "+what)
				}
			}
			return true
		})
	}
	return found
}

// TestFixedWidthTemplatesRideScreen fails a send of a boxed or column-aligned
// template on a category the pipeline wraps.
func TestFixedWidthTemplatesRideScreen(t *testing.T) {
	fset, files, _ := wrapGuardParse(t)
	var bad []string
	for _, gf := range files {
		for _, finding := range fixedWidthFindings(fset, gf.file) {
			bad = append(bad, gf.rel+": "+finding)
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("these sends fold a fixed-width layout at a narrow reader; send them on messaging.CategoryScreen:\n%s", strings.Join(bad, "\n"))
	}
}

// TestFixedWidthScreenGuard_CanFail is the null probe.
func TestFixedWidthScreenGuard_CanFail(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"table on System", `func f(u U) { s, _ := templates.Process("tables/generic", d, 1); u.SendText(messaging.CategorySystem, s) }`, 1},
		{"box on Refusal", `func f(u U) { s, _ := templates.Process("character/title", d, 1); u.SendText(messaging.CategoryRefusal, s) }`, 1},
		{"producer inline", `func f(u U) { u.SendText(messaging.CategorySystem, GetLockRender(a, b, 1)) }`, 1},
		{"producer through a variable", `func f(u U) { s := getAltTable(a, b, 1); u.SendText(messaging.CategorySystem, s) }`, 1},
		{"table on Screen", `func f(u U) { s, _ := templates.Process("tables/generic", d, 1); u.SendText(messaging.CategoryScreen, s) }`, 0},
		{"prose template keeps wrapping", `func f(u U) { s, _ := templates.Process("help/rename", d, 1); u.SendText(messaging.CategorySystem, s) }`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "probe.go", "package p\n"+tc.body+"\n", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := fixedWidthFindings(fset, f); len(got) != tc.want {
				t.Fatalf("findings %q, want %d", got, tc.want)
			}
		})
	}
}

// TestScreenKeepsAFixedWidthRowAt80 sends a status-style row of 81 visible
// columns the way status.go sends it, on CategoryScreen, and checks the player
// receives its padding intact at 80. The same row on CategorySystem folds,
// which is the probe that proves the comparison can fail.
func TestScreenKeepsAFixedWidthRowAt80(t *testing.T) {
	row := " Info: Ghostly Spirit" + strings.Repeat(" ", 81-len(" Info: Ghostly Spirit")-1) + "|"
	if len(row) != 81 {
		t.Fatalf("fixture row is %d wide, want 81", len(row))
	}
	render := func(cat messaging.Category) string {
		return messaging.RenderForRecipient(messaging.RenderInput{
			Category:  cat,
			Text:      row,
			Channel:   messaging.ChannelAudio,
			LineWidth: 80,
		})
	}
	if got := render(messaging.CategoryScreen); !strings.Contains(got, row) {
		t.Errorf("Screen changed an 81 column row at 80:\nwant it to contain %q\ngot  %q", row, got)
	}
	if got := render(messaging.CategorySystem); strings.Contains(got, row) {
		t.Errorf("System left an 81 column row whole at 80, so this test cannot tell the two apart: %q", got)
	}
}
