package guards

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// tokenEngineAllowed lists the only production files outside
// internal/narration that may hand a brace-delimited string literal to the
// strings.Replace family, with why. An entry here must be a NON-narration use
// of braces: a shell or SQL fragment, a JSON skeleton, a filesystem template.
// It must never read "this store renders its own tokens" -- that is precisely
// the second engine this guard exists to stop.
//
// EMPTY as of M4a task 8, and correctly so: production Go under internal/ and
// modules/ holds 59 strings.Replace/ReplaceAll/NewReplacer calls, exactly one
// of which is inside internal/narration (render.go's NewReplacer, the engine).
// Of the other 58 not one passes a literal containing "{" -- they replace
// underscores with spaces, CRLF with LF, quotes, ansi tags and zone-name
// separators. Adding an entry is a
// design decision to be argued in review; weakening the matcher is not an
// option. Both directions of this map were exercised when the guard was
// proven capable of failing -- see the header comment on TestNoSecondTokenEngine.
var tokenEngineAllowed = map[string]string{}

// TestNoSecondTokenEngine fails when any file outside internal/narration
// substitutes a brace token by hand. The messaging arc promised this guard in
// its "How we know it worked" list and never built it; three engines survived
// M0 to M3 as a result (items.SetTokenValue, grapplemessaging.RenderTemplate,
// hooks.substitute), each rendering the same stores through different code.
//
// It matches a call to strings.Replace, strings.ReplaceAll or
// strings.NewReplacer where any argument is a string literal containing "{".
// The walk is over the whole *ast.File, so the package-level
// `var x = strings.NewReplacer("{actor}", ...)` form is covered as well as
// calls inside function bodies: a var declaration's value is an expression
// like any other.
//
// WHAT THIS GUARD DOES NOT CLAIM. It recognises the strings.Replace family and
// nothing else, so a second engine built on regexp.ReplaceAllString with a
// `\{...\}` pattern, on bytes.Replace, or on a hand-written index scan over a
// strings.Builder would pass unnoticed. That is a deliberate, narrow matcher
// rather than an oversight: all three engines this arc actually deleted were
// strings.Replace-shaped, and a wider net would have to be argued against the
// 13 regexp-replacement sites under internal/ that are all legitimately
// non-narration (ansi tags, <name> tags, filename sanitising, a/an
// normalisation). A false claim of total coverage would be worse than this
// stated gap.
//
// Two token engines survive by design and are NOT matched, for different
// reasons:
//
//   - internal/narration itself, skipped by path. It is the one engine.
//   - internal/users/userrecord.prompt.go, the status-prompt engine. It is a
//     regexp (`\{[a-zA-Z%:\-]+\}`) plus a switch over a different vocabulary
//     ({hp}, {mp}, {tnl}), it renders a HUD rather than narration, and it
//     holds no strings.Replace call at all, so the matcher never reaches it.
//
// PROVEN CAPABLE OF FAILING. A scratch internal/hooks/zz_sabotage.go holding
// `strings.ReplaceAll(s, "{actor}", name)` was confirmed to compile under
// `go vet ./internal/hooks/` and then confirmed to turn this test red by name
// and line. The allowlist path was exercised in the same session: with the
// sabotage still present and its file registered in tokenEngineAllowed the
// test went green again, and with the entry left behind after the sabotage
// was deleted the test went red as stale. Re-verify the same three ways after
// any change to the matcher; a green run from a blind matcher proves nothing.
func TestNoSecondTokenEngine(t *testing.T) {
	var offenders []string
	seen := map[string]bool{}
	scanned := 0

	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					// A test elsewhere can create and remove a temp file under
					// the tree while packages test in parallel; a vanished
					// entry has nothing to scan.
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel := filepath.ToSlash(path)
			if strings.HasPrefix(rel, "internal/narration/") {
				return nil
			}
			fset := token.NewFileSet()
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return perr
			}
			scanned++
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "strings" {
					return true
				}
				switch sel.Sel.Name {
				case "Replace", "ReplaceAll", "NewReplacer":
				default:
					return true
				}
				for _, arg := range call.Args {
					lit, ok := arg.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING || !strings.Contains(lit.Value, "{") {
						continue
					}
					if _, allowed := tokenEngineAllowed[rel]; allowed {
						seen[rel] = true
						continue
					}
					offenders = append(offenders, fmt.Sprintf("%s:%d: %s.%s with the brace-token literal %s",
						rel, fset.Position(lit.Pos()).Line, pkg.Name, sel.Sel.Name, lit.Value))
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	// The allowlist is empty, so nothing above proves the walk saw the tree:
	// a walk that found no files would pass with no offenders.
	if scanned < 100 {
		t.Fatalf("parsed only %d Go files under %v; the walk is not seeing the tree, so a pass proves nothing", scanned, messagingSurfaceGoRoots)
	}

	// Every allowlist entry must still match. Without this the map rots into
	// coverage it no longer provides, and a stale entry could silently excuse
	// a file that later grows a real engine.
	for file, why := range tokenEngineAllowed {
		if !seen[file] {
			t.Errorf("allowlist entry %s (%s) no longer replaces a brace-token literal; remove it", file, why)
		}
	}

	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("%d hand-rolled token substitution(s) outside internal/narration:\n  %s\n\n"+
			"narration.Substitute is the single token engine (messaging arc M4a). Render "+
			"through it, or through the store's own narration.Render / textutil.Narrate "+
			"door. If this is genuinely not narration -- a shell, SQL or JSON fragment "+
			"that happens to contain a brace -- add the file to tokenEngineAllowed with a "+
			"one-line reason. Do not widen or weaken the matcher to make this pass.",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}
