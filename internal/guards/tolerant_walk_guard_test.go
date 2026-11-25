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

	"github.com/GoMudEngine/GoMud/internal/util"
)

// #953: packages test in parallel, and tests create and remove temp folders
// under the repo (internal/<pkg>/_datafiles/...). A test that walks the tree
// with a bare filepath.Walk or filepath.WalkDir fails whenever it lists one of
// those folders and the folder is gone before it is read. The rule that
// tolerates a vanished entry, and still fails on every real error, lives in
// one place: util.WalkDirTolerant and util.WalkTolerant. No test file may walk
// a tree any other way.

// bareWalkCalls returns "file:line filepath.X" for each bare tree walk in src.
func bareWalkCalls(fset *token.FileSet, file *ast.File) []string {
	local := map[string]bool{}
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || p != "path/filepath" {
			continue
		}
		name := "filepath"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		local[name] = true
	}
	if len(local) == 0 {
		return nil
	}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || !local[id.Name] || (sel.Sel.Name != "Walk" && sel.Sel.Name != "WalkDir") {
			return true
		}
		pos := fset.Position(call.Pos())
		out = append(out, pos.Filename+":"+strconv.Itoa(pos.Line)+" filepath."+sel.Sel.Name)
		return true
	})
	return out
}

func TestTestsWalkTreesThroughTheTolerantWalker(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	var offenders []string
	scanned := 0
	err := util.WalkDirTolerant(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		file, perr := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		scanned++
		for _, o := range bareWalkCalls(fset, file) {
			rel, rerr := filepath.Rel(root, o)
			if rerr != nil {
				rel = o
			}
			offenders = append(offenders, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if scanned < 500 {
		t.Fatalf("scanned only %d test files; the walk is not seeing the repo, so a pass proves nothing", scanned)
	}
	for _, o := range offenders {
		t.Errorf("%s: walk with util.WalkDirTolerant or util.WalkTolerant, which skip an entry another test removed mid-walk (#953)", o)
	}
}

// The guard can fail: a bare walk, an aliased import and a WalkDir all count,
// and a walk through the util helpers does not.
func TestTestsWalkTreesThroughTheTolerantWalker_GuardCanFail(t *testing.T) {
	src := `package x

import (
	fp "path/filepath"
	"path/filepath"

	"github.com/GoMudEngine/GoMud/internal/util"
)

func a() { _ = filepath.WalkDir(".", nil) }
func b() { _ = fp.Walk(".", nil) }
func c() { _ = util.WalkDirTolerant(".", nil) }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture_test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := bareWalkCalls(fset, file)
	if len(got) != 2 || !strings.HasSuffix(got[0], ":10 filepath.WalkDir") || !strings.HasSuffix(got[1], ":11 filepath.Walk") {
		t.Fatalf("got %v, want the WalkDir on line 10 and the aliased Walk on line 11", got)
	}
}
