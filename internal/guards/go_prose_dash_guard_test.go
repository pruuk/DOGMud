package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// House style for player text held in Go (M6 slice 2, ledger row 55): no em
// dash, en dash or spaced double hyphen in a string literal. Spaced single
// hyphens are not checked here: Go uses " - " in format and usage strings
// too often for a pattern to tell them from a dash.
//
// Skipped: string literals inside log, error, panic and test-failure calls;
// and the paths below, which hold builder, admin, validator or migration
// output, the dash-folding table, or quest and dialogue text (#375).
// The last alternative catches a " -- " used as a column separator after
// aligning spaces, as the old start menu did ("New to DOGMud      -- I know").
var goDash = regexp.MustCompile("[—–]|[A-Za-z0-9,.!?'\")>}] -- [A-Za-z0-9<{'\"%]|  -- [A-Za-z0-9<{'\"%]")

var fmtVerb = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z]`)

var goDashExemptPrefixes = []string{
	"internal/behaviortree/",
	"internal/conversations/",
	"internal/dialogue/",
	"internal/mapper/",
	"internal/migration/",
	"internal/questengine/",
	"internal/quests/",
	"modules/gmcp/",
}

var goDashExemptFiles = map[string]string{
	"internal/actions/divergences.go":           "registry audit log text",
	"internal/baubles/validate.go":              "the typography fold table",
	"internal/caravan/visit.go":                 "log text",
	"internal/darkcombat/store.go":              "the validator's own pattern",
	"internal/ferry/route.go":                   "log text",
	"internal/ferry/tradecircuit.go":            "loader error text",
	"internal/hooks/Quest_HandleQuestUpdate.go": "quest text, #375",
	"internal/lightnotice/store.go":             "the validator's own pattern",
	"internal/mobs/patrol_loader.go":            "loader log text",
	"internal/mobs/schedule_loader.go":          "loader log text",
	"internal/state/position/validation.go":     "invariant error text",
	"internal/util/util.go":                     "the dash-folding table",
	"modules/weather/weather_commands.go":       "admin command output",
}

func goDashExempt(rel string) bool {
	for _, p := range goDashExemptPrefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	if _, ok := goDashExemptFiles[rel]; ok {
		return true
	}
	return strings.HasPrefix(filepath.Base(rel), "admin.")
}

// quietCall reports a call whose string arguments are not player text.
func quietCall(c *ast.CallExpr) bool {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name == "panic"
	case *ast.SelectorExpr:
		if x, ok := f.X.(*ast.Ident); ok {
			switch x.Name {
			case "mudlog", "errors", "log", "slog":
				return true
			case "fmt":
				return f.Sel.Name == "Errorf"
			}
		}
	}
	return false
}

// rightmostStringLit returns the string literal that ends expression e, if e
// is a literal or a chain of literals joined by +.
func rightmostStringLit(e ast.Expr) *ast.BasicLit {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			return x
		}
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return rightmostStringLit(x.Y)
		}
	case *ast.ParenExpr:
		return rightmostStringLit(x.X)
	}
	return nil
}

// leftmostStringLit returns the string literal that begins expression e.
func leftmostStringLit(e ast.Expr) *ast.BasicLit {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			return x
		}
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return leftmostStringLit(x.X)
		}
	case *ast.ParenExpr:
		return leftmostStringLit(x.X)
	}
	return nil
}

func endsLowerLetter(s string) bool {
	return s != "" && s[len(s)-1] >= 'a' && s[len(s)-1] <= 'z'
}

func startsLowerLetter(s string) bool {
	return s != "" && s[0] >= 'a' && s[0] <= 'z'
}

func TestGoPlayerStringsHaveNoDashes(t *testing.T) {
	root := repoRoot(t)
	scanned := 0
	for _, top := range []string{"internal", "modules"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if goDashExempt(rel) {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			scanned++
			var quiet [][2]token.Pos
			ast.Inspect(f, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok && quietCall(c) {
					quiet = append(quiet, [2]token.Pos{c.Pos(), c.End()})
				}
				return true
			})
			inQuiet := func(from, to token.Pos) bool {
				for _, q := range quiet {
					if from >= q[0] && to <= q[1] {
						return true
					}
				}
				return false
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if b, ok := n.(*ast.BinaryExpr); ok && b.Op == token.ADD && !inQuiet(b.Pos(), b.End()) {
					left, right := rightmostStringLit(b.X), leftmostStringLit(b.Y)
					if left != nil && right != nil {
						ls, lerr := strconv.Unquote(left.Value)
						rs, rerr := strconv.Unquote(right.Value)
						if lerr == nil && rerr == nil && endsLowerLetter(ls) && startsLowerLetter(rs) {
							t.Errorf("%s:%d: two words run together across a +: %q + %q", rel, fset.Position(b.OpPos).Line, ls, rs)
						}
					}
				}
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if inQuiet(lit.Pos(), lit.End()) {
					return true
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					s = lit.Value
				}
				line := fset.Position(lit.Pos()).Line
				if goDash.MatchString(s) {
					t.Errorf("%s:%d: dash in a string: %s", rel, line, s)
				}
				// "%s<ansi ...>" is a format verb standing for text that ends in a
				// space or a tag, not a word run into a tag.
				if wordTagJoin.MatchString(fmtVerb.ReplaceAllString(s, "%")) {
					t.Errorf("%s:%d: word joined to a tag (needs a space): %s", rel, line, s)
				}
				if w := runTogetherPossessive(s); w != "" {
					t.Errorf("%s:%d: run-together possessive %q: %s", rel, line, w, s)
				}
				if joinedAreRun.MatchString(s) {
					t.Errorf("%s:%d: joined words (you are / they are / we are): %s", rel, line, s)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned only %d Go files; the walk is broken", scanned)
	}
}
