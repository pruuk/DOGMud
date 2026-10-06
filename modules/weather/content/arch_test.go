package content

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// allowedInternalImports carries the narrow exceptions to the "content never
// imports the engine" rule below.
//
// internal/narration is the messaging-unification arc's shared rendering
// core: it picks a coordinated index across roles and substitutes tokens, and
// imports nothing but stdlib plus internal/util's low-level helpers (no game
// state, no rooms/mobs/players). Weather is the arc's actorless store and
// item 9 joins it to that core the same way internal/items already does
// (and internal/itemvoices did until item behaviour slice 2 moved its
// pools into item trees), so this is a deliberate widening of the
// boundary, not a hole in it — every other internal/ package stays forbidden.
var allowedInternalImports = map[string]bool{
	"github.com/GoMudEngine/GoMud/internal/narration": true,
}

// TestContentPackageStaysPure: content parses module data and must never
// import the GoMud engine — engine access belongs in engine/. The one
// exception is internal/narration; see allowedInternalImports.
func TestContentPackageStaysPure(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(p, "GoMudEngine/GoMud/internal") && !allowedInternalImports[p] {
				t.Errorf("%s imports forbidden engine package %q (content must stay pure)", e.Name(), p)
			}
		}
	}
}
