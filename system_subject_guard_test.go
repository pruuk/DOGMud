package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Messaging M7 slice 4 (#371), ruling R4, rule N2: a line that opens with
// another creature and addresses the player ("Bandit catches you in the
// act!") is narration of something done to them, never a confirmation of
// their own action, so it does not ride CategorySystem. Checked on the text
// that leads the argument after messaging.CategorySystem (subjectLead: the
// slice 3 opener guard's leadLiteral, also reading through HideNames, a +
// concatenation with a name in it, and a string constant of the same file).
// A hit is converted, or the rule below is narrowed (as "with you" is, for
// track's own report); never allow-list a file.
// A colour tag may wrap the whole line ahead of the name; a name tag whose
// colour is chosen at run time (`<ansi fg="%s">%s</ansi>`) is a name too.
var otherSubject = regexp.MustCompile(`^(?:<ansi fg="[a-z-]+">)*(?:<ansi fg="(?:username|mobname|%s)">%s</ansi>|%s |Someone |Something )`)
var addressesYou = regexp.MustCompile(`(?i)\byour?\b`)
var subjectNarrowings = []*regexp.Regexp{
	regexp.MustCompile(`is in the room with you!$`), // track's result, the player's own report
	regexp.MustCompile(`^Something glints\b`),       // search's find: a thing, not a creature, the player's own report
}

// fileStrings maps each top-level string const or var of a file, written as
// a literal, to its text.
func fileStrings(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
			continue
		}
		for _, sp := range gd.Specs {
			vs, ok := sp.(*ast.ValueSpec)
			if !ok || len(vs.Names) != len(vs.Values) {
				continue
			}
			for i, n := range vs.Names {
				if s, ok := leadLiteral(vs.Values[i]); ok {
					out[n.Name] = s
				}
			}
		}
	}
	return out
}

// subjectLead is the text a send argument opens with. A + concatenation is
// read part by part, a part no literal decides standing as %s (a name), so
// `<ansi fg="username">`+m.Name+`</ansi> appears` reads as the Sprintf
// form; HideNames is read through to the line it hides names in.
func subjectLead(e ast.Expr, consts map[string]string) (string, bool) {
	switch v := e.(type) {
	case *ast.Ident:
		s, ok := consts[v.Name]
		return s, ok
	case *ast.CallExpr:
		if calledName(v) == `HideNames` && len(v.Args) > 0 {
			return subjectLead(v.Args[0], consts)
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			l, lok := subjectLead(v.X, consts)
			r, rok := subjectLead(v.Y, consts)
			if !lok && !rok {
				return ``, false
			}
			if !lok {
				l = `%s`
			}
			if !rok {
				r = `%s`
			}
			return l + r, true
		}
	case *ast.ParenExpr:
		return subjectLead(v.X, consts)
	}
	return leadLiteral(e)
}

func systemSubjectHits(t *testing.T, rel string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	consts := fileStrings(f)
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
			s, ok := subjectLead(call.Args[i+1], consts)
			if !ok || !otherSubject.MatchString(strings.TrimSpace(s)) {
				continue
			}
			plain := openerTag.ReplaceAllString(s, ``)
			if !addressesYou.MatchString(plain) {
				continue
			}
			narrowed := false
			for _, nr := range subjectNarrowings {
				if nr.MatchString(strings.TrimSpace(plain)) {
					narrowed = true
				}
			}
			if !narrowed {
				hits = append(hits, rel+`:`+strconv.Itoa(fset.Position(call.Pos()).Line)+`: `+plain)
			}
		}
		return true
	})
	return hits
}

func TestNarrationAboutYouNeverRidesSystem(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal(`runtime.Caller failed`)
	}
	root := filepath.Dir(here)
	scanned := 0
	for _, top := range []string{`internal`, `modules`} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, `.go`) || strings.HasSuffix(path, `_test.go`) {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, h := range systemSubjectHits(t, filepath.ToSlash(rel), src) {
				t.Errorf(`%s (narration about the player: give it its narration category)`, h)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 100 {
		t.Fatalf(`scanned only %d Go files; the walk is broken`, scanned)
	}
}

func TestSystemSubjectGuardCanFail(t *testing.T) {
	src := "package p\n" +
		"func a(u U) { u.SendText(messaging.CategorySystem, fmt.Sprintf(`%s catches you in the act!`, n)) }\n" +
		"func b(u U) { u.SendText(messaging.CategorySystem, `<ansi fg=\"mobname\">%s</ansi> turns on you with fury!`) }\n" +
		"func c(u U) { u.SendText(messaging.CategorySystem, `Someone is attacking you.`) }\n" +
		"func d(u U) { u.SendText(messaging.CategorySystem, `<ansi fg=\"mobname\">%s</ansi> is now known as %s.`) }\n" +
		"func e(u U) { u.SendText(messaging.CategoryMobEmote, `%s catches you in the act!`) }\n" +
		"func f(u U) { u.SendText(messaging.CategorySystem, `%s is in the room with you!`) }\n" +
		"func g(u U) { u.SendText(messaging.CategorySystem, `<ansi fg=\"username\">`+m.Name+`</ansi> appears to help you out!`) }\n" +
		"func h(u U) { u.SendText(messaging.CategorySystem, messaging.HideNames(fmt.Sprintf(`<ansi fg=\"red\"><ansi fg=\"%s\">%s</ansi> blocks you!</ansi>`, tag, n), nil, s)) }\n" +
		"const cue = `Someone is attacking you.`\n" +
		"func i(u U) { u.SendText(messaging.CategorySystem, cue) }\n" +
		"func j(u U) { u.SendText(messaging.CategorySystem, name+` has gone their own way.`) }\n" +
		"func k(u U) { u.SendText(messaging.CategorySystem, `Something glints among the clutter. You set about working it loose...`) }\n"
	hits := systemSubjectHits(t, `probe.go`, []byte(src))
	want := []string{`probe.go:2:`, `probe.go:3:`, `probe.go:4:`, `probe.go:8:`, `probe.go:9:`, `probe.go:11:`}
	if len(hits) != len(want) {
		t.Fatalf(`want %d hits, got %v`, len(want), hits)
	}
	for i, w := range want {
		if !strings.HasPrefix(hits[i], w) {
			t.Fatalf(`hit %d: want %s, got %s`, i, w, hits[i])
		}
	}
}
