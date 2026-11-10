package guards

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Web client base, ruling 1: all wrapping goes through one place and follows
// the device width. UserRecord.WrapWidth is that place. These guards fail a
// literal or constant width handed to a wrap function, a literal width on a
// RenderInput, and any new read of the saved LineWidth where a wrap width was
// meant.

// wrapWidthFuncs names every function that wraps text, and which argument is
// the width.
var wrapWidthFuncs = map[string]int{
	"SplitString":          1,
	"SplitStringNL":        1,
	"WrapAnsi":             1,
	"wrap":                 1, // internal/menu and internal/messaging
	"wrapPrompt":           1, // internal/usercommands/set.go
	"wrapDescriptionParts": 1, // internal/rooms/roomdetails.go
}

// wrapWidthExempt is every fixed width allowed to reach a wrap function, keyed
// on file and the printed width expression. Authoring limits that reject a
// long line (lightnotice/store.go, darkcombat/store.go), a table's own column
// budget (online.go) and a hint's length check (sight_aim.go) never reach a
// wrap function, so they need no entry here.
var wrapWidthExempt = map[string]string{
	"internal/usercommands/setdesc.go|72":              "echoes the description at the 72 columns the character template shows it; a template layout, plan 3's to change",
	"internal/usercommands/admin.server.go|lineLength": "the admin config dump, laid out to 158 columns less the longest key name; staff tooling on a wide screen",
}

// getLineWidthAllowed is who may read the saved width directly.
var getLineWidthAllowed = map[string]string{
	"internal/users/wrapwidth.go":  "WrapWidth itself",
	"internal/usercommands/set.go": "`set linewidth` reports the saved setting, which is not a wrap",
}

// wrapConstIndex is every const declared under internal/ and modules/: bare
// names per directory, and "pkg.Name" for a qualified use.
type wrapConstIndex struct {
	byDir map[string]map[string]bool
	byPkg map[string]bool
}

func newWrapConstIndex() *wrapConstIndex {
	return &wrapConstIndex{byDir: map[string]map[string]bool{}, byPkg: map[string]bool{}}
}

func (x *wrapConstIndex) add(dir string, f *ast.File) {
	if x.byDir[dir] == nil {
		x.byDir[dir] = map[string]bool{}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		d, ok := n.(*ast.GenDecl)
		if !ok || d.Tok != token.CONST {
			return true
		}
		for _, s := range d.Specs {
			for _, name := range s.(*ast.ValueSpec).Names {
				x.byDir[dir][name.Name] = true
				x.byPkg[f.Name.Name+"."+name.Name] = true
			}
		}
		return true
	})
}

// wrapWidthIsFixed reports whether expr has the same value for every reader:
// a literal, a constant, len() of something (a layout's own overhead, which
// says nothing about the reader), or arithmetic of those.
func wrapWidthIsFixed(expr ast.Expr, dir string, consts *wrapConstIndex, fn *ast.FuncDecl) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.ParenExpr:
		return wrapWidthIsFixed(e.X, dir, consts, fn)
	case *ast.BinaryExpr:
		return wrapWidthIsFixed(e.X, dir, consts, fn) && wrapWidthIsFixed(e.Y, dir, consts, fn)
	case *ast.CallExpr:
		id, ok := e.Fun.(*ast.Ident)
		return ok && id.Name == "len"
	case *ast.SelectorExpr:
		pkg, ok := e.X.(*ast.Ident)
		return ok && consts.byPkg[pkg.Name+"."+e.Sel.Name]
	case *ast.Ident:
		if consts.byDir[dir][e.Name] {
			return true
		}
		return localOnlyAssignedFixed(e.Name, dir, consts, fn)
	}
	return false
}

// localOnlyAssignedFixed reports whether name is a local of fn whose every
// assignment is fixed, so `w := 80` cannot launder a literal. A parameter, or
// a name fn never assigns, is not fixed.
func localOnlyAssignedFixed(name, dir string, consts *wrapConstIndex, fn *ast.FuncDecl) bool {
	if fn == nil {
		return false
	}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, id := range field.Names {
				if id.Name == name {
					return false
				}
			}
		}
	}
	values := []ast.Expr{}
	allFixed := true
	ast.Inspect(fn, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
					if len(s.Rhs) != len(s.Lhs) {
						allFixed = false
						continue
					}
					values = append(values, s.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			for i, id := range s.Names {
				if id.Name == name && i < len(s.Values) {
					values = append(values, s.Values[i])
				}
			}
		}
		return true
	})
	if !allFixed || len(values) == 0 {
		return false
	}
	for _, v := range values {
		if id, ok := v.(*ast.Ident); ok && id.Name == name {
			return false
		}
		if !wrapWidthIsFixed(v, dir, consts, fn) {
			return false
		}
	}
	return true
}

func wrapCalleeName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

func wrapExprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return "?"
	}
	return buf.String()
}

// wrapWidthFindings is every fixed width in f, as "printed width expression".
func wrapWidthFindings(fset *token.FileSet, f *ast.File, dir string, consts *wrapConstIndex) []string {
	var found []string
	for _, decl := range f.Decls {
		fn, _ := decl.(*ast.FuncDecl)
		ast.Inspect(decl, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				idx, ok := wrapWidthFuncs[wrapCalleeName(node)]
				if ok && idx < len(node.Args) && wrapWidthIsFixed(node.Args[idx], dir, consts, fn) {
					found = append(found, wrapExprString(fset, node.Args[idx]))
				}
			case *ast.CompositeLit:
				if !strings.HasSuffix(wrapExprString(fset, node.Type), "RenderInput") {
					return true
				}
				for _, el := range node.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "LineWidth" && wrapWidthIsFixed(kv.Value, dir, consts, fn) {
						found = append(found, wrapExprString(fset, kv.Value))
					}
				}
			}
			return true
		})
	}
	return found
}

type wrapGuardFile struct {
	rel  string
	dir  string
	file *ast.File
}

// wrapGuardParse parses every production Go file under internal/ and modules/.
func wrapGuardParse(t *testing.T) (*token.FileSet, []wrapGuardFile, *wrapConstIndex) {
	t.Helper()
	fset := token.NewFileSet()
	consts := newWrapConstIndex()
	var files []wrapGuardFile
	for _, root := range []string{"internal", "modules"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil // a syntax error is the compiler's to report
			}
			dir := filepath.ToSlash(filepath.Dir(path))
			consts.add(dir, f)
			files = append(files, wrapGuardFile{rel: filepath.ToSlash(path), dir: dir, file: f})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(files) < 1000 {
		t.Fatalf("parsed only %d Go files: the walk is not reaching the code, so this guard proves nothing", len(files))
	}
	return fset, files, consts
}

// TestNoFixedWrapWidths is the guard. Use the reader's WrapWidth, or add the
// site to wrapWidthExempt with the reason it has no reader to follow.
func TestNoFixedWrapWidths(t *testing.T) {
	fset, files, consts := wrapGuardParse(t)
	seen := map[string]bool{}
	for _, gf := range files {
		for _, width := range wrapWidthFindings(fset, gf.file, gf.dir, consts) {
			key := gf.rel + "|" + width
			seen[key] = true
			if _, ok := wrapWidthExempt[key]; !ok {
				t.Errorf("%s wraps at the fixed width %q: pass the reader's WrapWidth(), or add %q to wrapWidthExempt with the reason", gf.rel, width, key)
			}
		}
	}
	stale := []string{}
	for key := range wrapWidthExempt {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		t.Errorf("wrapWidthExempt entry %q matches nothing: remove it", key)
	}
}

// TestOnlyWrapWidthReadsTheSavedLineWidth keeps GetLineWidth from becoming a
// second source of a wrap width.
func TestOnlyWrapWidthReadsTheSavedLineWidth(t *testing.T) {
	_, files, _ := wrapGuardParse(t)
	seen := map[string]bool{}
	for _, gf := range files {
		ast.Inspect(gf.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "GetLineWidth" {
				seen[gf.rel] = true
				if _, ok := getLineWidthAllowed[gf.rel]; !ok {
					t.Errorf("%s calls GetLineWidth; a wrap width comes from WrapWidth()", gf.rel)
				}
			}
			return true
		})
	}
	for rel := range getLineWidthAllowed {
		if !seen[rel] {
			t.Errorf("getLineWidthAllowed entry %q no longer calls GetLineWidth: remove it", rel)
		}
	}
}

// TestWrapWidthGuard_CanFail is the null probe: each shape a fixed width can
// take fires, and each reader-derived shape does not.
func TestWrapWidthGuard_CanFail(t *testing.T) {
	const header = "package p\nconst fixed = 80\nconst margin = 7\n"
	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"literal", `func f(s string) { util.SplitStringNL(s, 80) }`, []string{"80"}},
		{"WrapAnsi literal", `func f(s string) { messaging.WrapAnsi(s, 72) }`, []string{"72"}},
		{"named constant", `func f(s string) { util.SplitStringNL(s, fixed) }`, []string{"fixed"}},
		{"qualified constant", `func f(s string) { util.SplitStringNL(s, p.fixed) }`, []string{"p.fixed"}},
		{"constant arithmetic", `func f(s string) { util.SplitStringNL(s, fixed-margin) }`, []string{"fixed - margin"}},
		{"constant less a length", `func f(s, i string) { wrapPrompt(s, fixed-len(i), i) }`, []string{"fixed - len(i)"}},
		{"local set to a literal", `func f(s string) { w := 80; util.SplitStringNL(s, w) }`, []string{"w"}},
		{"unqualified inside its package", `func f(s string) { SplitString(s, 80) }`, []string{"80"}},
		{"RenderInput literal", `func f() { _ = messaging.RenderInput{LineWidth: 80} }`, []string{"80"}},
		{"reader's width", `func f(s string, u *U) { util.SplitStringNL(s, u.WrapWidth()) }`, nil},
		{"parameter", `func f(s string, width int) { util.SplitStringNL(s, width) }`, nil},
		{"parameter with a floor", `func f(s string, lw int) { if lw < 20 { lw = 20 }; w := lw - 4; util.SplitStringNL(s, w-1) }`, nil},
		{"parameter less a length", `func f(s, i string, width int) { util.SplitStringNL(s, width-len(i), i) }`, nil},
		{"local from the reader", `func f(s string, u *U) { w := u.WrapWidth() - 7; util.SplitStringNL(s, w-1) }`, nil},
		{"RenderInput from the reader", `func f(u *U) { _ = messaging.RenderInput{LineWidth: u.WrapWidth()} }`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "probe.go", header+tc.body+"\n", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			consts := newWrapConstIndex()
			consts.add("probe", f)
			got := wrapWidthFindings(fset, f, "probe", consts)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("findings %q, want %q", got, tc.want)
			}
		})
	}
}
