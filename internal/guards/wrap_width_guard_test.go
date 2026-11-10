package guards

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Web client base, ruling 1: all wrapping goes through one place and follows
// the device width. UserRecord.WrapWidth is that place. These guards fail a
// literal or constant width handed to a wrap function, a literal width on a
// RenderInput, and any new read of the saved LineWidth where a wrap width was
// meant.
//
// What the guard does not follow, because following it would need type
// information or flow analysis out of proportion to the risk:
//   - a local set to a fixed width and reassigned from the reader on one
//     branch (`w := 80; if web { w = u.WrapWidth() }`) reads as reader-derived;
//   - a wrap function taken as a value (`g := util.SplitStringNL; g(s, 80)`);
//   - a conversion to a named type (`Columns(80)`), and a constant reached
//     through a dot import.

// wrapWidthFuncs names every function that wraps text for a reader, and which
// argument is the width.
var wrapWidthFuncs = map[string]int{
	"SplitString":          1,
	"SplitStringNL":        1,
	"WrapAnsi":             1,
	"wrap":                 1, // internal/menu and internal/messaging
	"wrapPrompt":           1, // internal/usercommands/set.go
	"wrapDescriptionParts": 1, // internal/rooms/roomdetails.go
	"motdBox":              1, // internal/usercommands/motd.go
}

// wrapWidthPackageFuncs is the same for a name too common to match alone,
// keyed on package directory and name: gossip.Render and narration.Render take
// no width.
var wrapWidthPackageFuncs = map[string]int{
	"internal/menu.Render": 1,
}

const wrapGuardModulePrefix = "github.com/GoMudEngine/GoMud/"

// wrapWidthConversions are the conversions that leave a fixed width fixed.
var wrapWidthConversions = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
}

// wrapWidthExempt is every fixed width allowed to reach a wrap function, keyed
// on file and the printed width expression. Authoring limits that reject a
// long line (lightnotice/store.go, darkcombat/store.go), a table's own column
// budget (online.go) and a hint's length check (sight_aim.go) never reach a
// wrap function, so they need no entry here.
var wrapWidthExempt = map[string]string{
	"internal/usercommands/admin.server.go|lineLength": "the admin config dump, laid out to 158 columns less the longest key name; staff tooling on a wide screen",
}

// getLineWidthAllowed is who may read the saved width directly.
var getLineWidthAllowed = map[string]string{
	"internal/users/wrapwidth.go":  "WrapWidth itself",
	"internal/usercommands/set.go": "`set linewidth` reports the saved setting, which is not a wrap",
}

// wrapConstIndex is every package-level const and var declared under internal/
// and modules/: bare names per directory, and "pkg.Name" for a qualified use.
// A package-level var counts because it holds one value for every reader,
// however it was set.
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
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok || (d.Tok != token.CONST && d.Tok != token.VAR) {
			continue
		}
		for _, s := range d.Specs {
			for _, name := range s.(*ast.ValueSpec).Names {
				x.byDir[dir][name.Name] = true
				x.byPkg[f.Name.Name+"."+name.Name] = true
			}
		}
	}
}

// wrapScope is where a width expression is being read.
type wrapScope struct {
	dir     string
	consts  *wrapConstIndex
	imports map[string]string // local package name to directory; "" outside the module
	fn      *ast.FuncDecl
	reading map[string]bool // locals being followed, so a cycle of them ends
}

// wrapFileImports maps each import's local name in f to its directory.
func wrapFileImports(f *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range f.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		local := path.Base(importPath)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		imports[local] = ""
		if strings.HasPrefix(importPath, wrapGuardModulePrefix) {
			imports[local] = strings.TrimPrefix(importPath, wrapGuardModulePrefix)
		}
	}
	return imports
}

// wrapWidthIsFixed reports whether expr has the same value for every reader:
// a literal, a constant, a package-level var, a numeric conversion of one of
// those, len() of something (a layout's own overhead, which says nothing about
// the reader), or arithmetic of those.
func wrapWidthIsFixed(expr ast.Expr, sc wrapScope) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.ParenExpr:
		return wrapWidthIsFixed(e.X, sc)
	case *ast.BinaryExpr:
		return wrapWidthIsFixed(e.X, sc) && wrapWidthIsFixed(e.Y, sc)
	case *ast.CallExpr:
		id, ok := e.Fun.(*ast.Ident)
		if !ok {
			return false
		}
		if id.Name == "len" {
			return true
		}
		return wrapWidthConversions[id.Name] && len(e.Args) == 1 && wrapWidthIsFixed(e.Args[0], sc)
	case *ast.SelectorExpr:
		pkg, ok := e.X.(*ast.Ident)
		if !ok || wrapNameIsLocal(pkg.Name, sc.fn) {
			return false
		}
		if dir, imported := sc.imports[pkg.Name]; imported {
			return sc.consts.byDir[dir][e.Sel.Name]
		}
		return sc.consts.byPkg[pkg.Name+"."+e.Sel.Name]
	case *ast.Ident:
		if wrapNameIsLocal(e.Name, sc.fn) {
			return localOnlyAssignedFixed(e.Name, sc)
		}
		return sc.consts.byDir[sc.dir][e.Name]
	}
	return false
}

// wrapNameIsLocal reports whether fn binds name itself: its receiver, a
// parameter or result (a closure's too), a := or a local var or const.
func wrapNameIsLocal(name string, fn *ast.FuncDecl) bool {
	if fn == nil {
		return false
	}
	found := false
	checkFields := func(fields *ast.FieldList) {
		if fields == nil {
			return
		}
		for _, field := range fields.List {
			for _, id := range field.Names {
				found = found || id.Name == name
			}
		}
	}
	checkExpr := func(expr ast.Expr) {
		if id, ok := expr.(*ast.Ident); ok {
			found = found || id.Name == name
		}
	}
	checkFields(fn.Recv)
	ast.Inspect(fn, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.FuncType:
			checkFields(s.Params)
			checkFields(s.Results)
		case *ast.AssignStmt:
			if s.Tok == token.DEFINE {
				for _, lhs := range s.Lhs {
					checkExpr(lhs)
				}
			}
		case *ast.RangeStmt:
			if s.Tok == token.DEFINE {
				checkExpr(s.Key)
				checkExpr(s.Value)
			}
		case *ast.ValueSpec:
			for _, id := range s.Names {
				found = found || id.Name == name
			}
		}
		return !found
	})
	return found
}

// localOnlyAssignedFixed reports whether name is a local of fn whose every
// assignment is fixed, so `w := 80` cannot launder a literal. A parameter, a
// name fn never assigns, or locals assigned only from each other are not
// fixed.
func localOnlyAssignedFixed(name string, sc wrapScope) bool {
	fn := sc.fn
	if fn == nil || sc.reading[name] {
		return false
	}
	sc.reading[name] = true
	defer delete(sc.reading, name)
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
	isConst := false
	ast.Inspect(fn, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.GenDecl:
			if s.Tok == token.CONST {
				for _, spec := range s.Specs {
					for _, id := range spec.(*ast.ValueSpec).Names {
						isConst = isConst || id.Name == name
					}
				}
			}
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
	if isConst {
		return true
	}
	if !allFixed || len(values) == 0 {
		return false
	}
	for _, v := range values {
		if !wrapWidthIsFixed(v, sc) {
			return false
		}
	}
	return true
}

// wrapWidthArg returns the width argument's index when call is a wrap function.
func wrapWidthArg(call *ast.CallExpr, sc wrapScope) (int, bool) {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		if idx, ok := wrapWidthFuncs[f.Name]; ok {
			return idx, true
		}
		idx, ok := wrapWidthPackageFuncs[sc.dir+"."+f.Name]
		return idx, ok
	case *ast.SelectorExpr:
		if idx, ok := wrapWidthFuncs[f.Sel.Name]; ok {
			return idx, true
		}
		pkg, ok := f.X.(*ast.Ident)
		if !ok || wrapNameIsLocal(pkg.Name, sc.fn) {
			return 0, false
		}
		idx, ok := wrapWidthPackageFuncs[sc.imports[pkg.Name]+"."+f.Sel.Name]
		return idx, ok
	}
	return 0, false
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
	imports := wrapFileImports(f)
	for _, decl := range f.Decls {
		fn, _ := decl.(*ast.FuncDecl)
		sc := wrapScope{dir: dir, consts: consts, imports: imports, fn: fn, reading: map[string]bool{}}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				idx, ok := wrapWidthArg(node, sc)
				if ok && idx < len(node.Args) && wrapWidthIsFixed(node.Args[idx], sc) {
					found = append(found, wrapExprString(fset, node.Args[idx]))
				}
			case *ast.AssignStmt:
				if node.Tok != token.ASSIGN || len(node.Lhs) != len(node.Rhs) {
					return true
				}
				for i, lhs := range node.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "LineWidth" && wrapWidthIsFixed(node.Rhs[i], sc) {
						found = append(found, wrapExprString(fset, node.Rhs[i]))
					}
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
					if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "LineWidth" && wrapWidthIsFixed(kv.Value, sc) {
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
	const header = "package p\n" +
		"import (\n" +
		"\tly \"github.com/GoMudEngine/GoMud/internal/layout\"\n" +
		"\t\"github.com/GoMudEngine/GoMud/internal/menu\"\n" +
		"\tmn \"github.com/GoMudEngine/GoMud/internal/menu\"\n" +
		")\n" +
		"const fixed = 80\nconst margin = 7\nvar shared = 80\n"
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

		{"menu.Render literal", `func f(m M) { menu.Render(m, 80) }`, []string{"80"}},
		{"menu.Render through an alias", `func f(m M) { mn.Render(m, fixed) }`, []string{"fixed"}},
		{"Render inside the menu package", `func f(m M) { Render(m, 80) }`, []string{"80"}},
		{"menu.Render from the reader", `func f(m M, u *U) { menu.Render(m, u.WrapWidth()) }`, nil},
		{"another package's Render", `func f(p P) { gossip.Render(p, 80, "") }`, nil},
		{"a bare Render elsewhere", `func f(p P) { Render(p, 80) }`, nil},
		{"motdBox literal", `func f(s string) { motdBox(s, 80) }`, []string{"80"}},
		{"motdBox from the reader", `func f(s string, u *U) { motdBox(s, u.WrapWidth()) }`, nil},
		{"package-level var", `func f(s string) { util.SplitStringNL(s, shared) }`, []string{"shared"}},
		{"package-level var in a sum", `func f(s string) { util.SplitStringNL(s, shared-margin) }`, []string{"shared - margin"}},
		{"another package's var", `func f(s string) { util.SplitStringNL(s, ly.Cols) }`, []string{"ly.Cols"}},
		{"local shadows the package var", `func f(s string, u *U) { shared := u.WrapWidth(); util.SplitStringNL(s, shared) }`, nil},
		{"parameter shadows the constant", `func f(s string, fixed int) { util.SplitStringNL(s, fixed) }`, nil},
		{"conversion of a literal", `func f(s string) { util.SplitStringNL(s, int(80)) }`, []string{"int(80)"}},
		{"conversion of a constant", `func f(s string) { util.SplitStringNL(s, int(fixed)) }`, []string{"int(fixed)"}},
		{"conversion of the reader's width", `func f(s string, u *U) { util.SplitStringNL(s, int(u.WrapWidth())) }`, nil},
		{"constant through an alias", `func f(s string) { util.SplitStringNL(s, ly.Width) }`, []string{"ly.Width"}},
		{"alias shadowed by a parameter", `func f(s string, ly U) { util.SplitStringNL(s, ly.Width) }`, nil},
		{"field set to a literal", `func f(in *messaging.RenderInput) { in.LineWidth = 80 }`, []string{"80"}},
		{"field set to a constant", `func f(in *messaging.RenderInput) { in.LineWidth = fixed - margin }`, []string{"fixed - margin"}},
		{"field set from the reader", `func f(in *messaging.RenderInput, u *U) { in.LineWidth = u.WrapWidth() }`, nil},
		{"locals assigned from each other", `func f(s string) { a, b := 1, 2; a = b; b = a; util.SplitStringNL(s, a) }`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := "probe"
			if tc.name == "Render inside the menu package" {
				dir = "internal/menu"
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "probe.go", header+tc.body+"\n", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			layout, err := parser.ParseFile(fset, "layout.go", "package layout\nconst Width = 80\nvar Cols = 80\n", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			consts := newWrapConstIndex()
			consts.add(dir, f)
			consts.add("internal/layout", layout)
			got := wrapWidthFindings(fset, f, dir, consts)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("findings %q, want %q", got, tc.want)
			}
		})
	}
}
