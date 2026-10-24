package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Messaging M7 slice 3 (#371): a line that opens like a refusal rides
// messaging.CategoryRefusal, never CategorySystem. The guard looks at every
// call that passes messaging.CategorySystem and reads the argument after it
// (a string literal, the format of fmt.Sprintf, or the left end of a +
// concatenation), strips tags and tests it against refusalOpener. The
// slice 3 ledger (docs/superpowers/audits/messaging-m7-refusal-ledger.md)
// rules on the rest, which no pattern can tell apart from a report.
//
// A hit is converted, or refusalOpener is narrowed; never allow-list a file.
// A line holding ", but " reports a partial success ("You don't have that
// much gold on hand, but what you do have you deposit."), not a refusal.
var refusalOpener = regexp.MustCompile(`^(You (can't|cannot|don't have|aren't|are not|need to|must|'re too|are too|'re already|are already|have no)|Only (the|officers) )`)

var openerTag = regexp.MustCompile(`<[^>]*>`)

// openerGuardScope is every non-test Go file under internal/ and modules/
// (slice 3 widened it group by group; it now covers the whole tree).
var openerGuardScope = []string{"internal/", "modules/"}

func inOpenerScope(rel string) bool {
	for _, p := range openerGuardScope {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}

// leadLiteral is the text a send argument opens with, when a literal decides it.
func leadLiteral(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return ``, false
		}
		s, err := strconv.Unquote(v.Value)
		if err != nil {
			return v.Value, true
		}
		return s, true
	case *ast.CallExpr:
		if calledName(v) == `Sprintf` && len(v.Args) > 0 {
			return leadLiteral(v.Args[0])
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return leadLiteral(v.X)
		}
	case *ast.ParenExpr:
		return leadLiteral(v.X)
	}
	return ``, false
}

func systemOpenerHits(t *testing.T, rel string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for i, a := range call.Args {
			sel, ok := a.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != `CategorySystem` || i+1 >= len(call.Args) {
				continue
			}
			s, ok := leadLiteral(call.Args[i+1])
			if !ok {
				continue
			}
			plain := strings.TrimSpace(openerTag.ReplaceAllString(s, ``))
			if refusalOpener.MatchString(plain) && !strings.Contains(plain, `, but `) {
				hits = append(hits, rel+`:`+strconv.Itoa(fset.Position(call.Pos()).Line)+`: `+plain)
			}
		}
		return true
	})
	return hits
}

func TestRefusalOpenersNeverRideSystem(t *testing.T) {
	root := repoRoot(t)
	scanned := 0
	for _, top := range []string{`internal`, `modules`} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, `.go`) || strings.HasSuffix(path, `_test.go`) {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if !inOpenerScope(rel) {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, h := range systemOpenerHits(t, rel, src) {
				t.Errorf(`%s (send it on messaging.CategoryRefusal, or use internal/refusal)`, h)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 50 {
		t.Fatalf(`scanned only %d Go files; the walk or the scope is broken`, scanned)
	}
}

// TestRefusalOpenerGuardCanFail feeds the finder each send shape.
func TestRefusalOpenerGuardCanFail(t *testing.T) {
	src := "package p\n" +
		"func a(u U) { u.SendText(messaging.CategorySystem, `You can't do that.`) }\n" +
		"func b(u U) { u.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg=\"red\">You aren't carrying %s.</ansi>`, x)) }\n" +
		"func c(u U) { u.SendText(messaging.CategorySystem, `Only the guild leader can `+x) }\n" +
		"func d(u U) { craftDeliver(u, messaging.CategorySystem, `You need to be at a forge.`) }\n" +
		"func e(u U) { u.SendText(messaging.CategoryRefusal, `You can't do that.`) }\n" +
		"func f(u U) { u.SendText(messaging.CategorySystem, `You don't have that much gold on hand, but you deposit it.`) }\n" +
		"func g(u U) { u.SendText(messaging.CategorySystem, `Only you may now withdraw gold.`) }\n" +
		"func h(u U) { u.SendText(messaging.CategorySystem, `You pick up the sword.`) }\n"
	hits := systemOpenerHits(t, `probe.go`, []byte(src))
	want := []string{`probe.go:2:`, `probe.go:3:`, `probe.go:4:`, `probe.go:5:`}
	if len(hits) != len(want) {
		t.Fatalf(`want %d hits, got %v`, len(want), hits)
	}
	for i, w := range want {
		if !strings.HasPrefix(hits[i], w) {
			t.Fatalf(`hit %d: want %s, got %s`, i, w, hits[i])
		}
	}
}
