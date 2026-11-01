package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Player text reaches telnet and the web client through ansitags, which
// recognises only <ansi ...> and </ansi> and passes every other byte through
// as written (internal/util/ansiescape.go). Nothing on the way decodes an
// HTML entity, so "talk &lt;name&gt;" reached players literally (#690).
// Write the angle brackets themselves: "talk <name>".
var htmlEntityInGo = regexp.MustCompile(`&(lt|gt);`)

// htmlEntityExempt lists files whose literal is HTML, where the entity is right.
var htmlEntityExempt = map[string]string{
	"internal/playtestenv/report.go": "writes an HTML report",
}

func TestPlayerTextHasNoHTMLEntities(t *testing.T) {
	if !htmlEntityInGo.MatchString(`talk &lt;name&gt;`) {
		t.Fatal("probe: the pattern cannot match an entity")
	}
	root := repoRoot(t)
	fset := token.NewFileSet()
	scanned := 0
	for _, dir := range []string{"internal", "modules"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator)))
			if _, ok := htmlEntityExempt[rel]; ok {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return perr
			}
			scanned++
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if ok && lit.Kind == token.STRING && htmlEntityInGo.MatchString(lit.Value) {
					t.Errorf("%s: an HTML entity reaches the player literally; write < and >: %s",
						fset.Position(lit.Pos()), lit.Value)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	if scanned < 500 {
		t.Fatalf("scanned only %d Go files; the walk is not reaching the code", scanned)
	}
}
