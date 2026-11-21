package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The game has one word wrapper: internal/messaging (WrapAnsi, and WrapPrompt
// for a prompt). Owner rule, 2026-11-21: "We UNIFY code here. Check for
// existing code to wire into instead of reimplementing." The #842 pager shipped
// its own text measuring beside it and internal/menu had its own word wrapper,
// so this guard makes a new one outside internal/messaging fail the build.
//
// The detection rule, applied to every function declaration in a non-test Go
// file under internal/ and modules/:
//
//  1. Its name matches oneWrapperName: (?i)(wrap|splitstring).
//  2. It is not in internal/messaging, the wrapper's home.
//  3. It produces text: one of its results is string or []string. This drops,
//     by rule and with no list, every "wrap" name that has nothing to do with
//     text: Unwrap() error on the error types, rooms.wrapCoord (int),
//     behaviortree.wrapsProcAlone (bool), the WrapWidth readers (int) and
//     characters.MigrateDescriptionWrapping (no result).
//  4. It is not a delegator. A delegator has no for or range statement
//     anywhere in its body (so it measures nothing itself) and calls
//     messaging.WrapAnsi or messaging.WrapPrompt, or a function of its own
//     package that is itself a delegator (templates.wrapSpans calls
//     templates.wrapText, which calls WrapAnsi). menu.wrapLines and
//     templates.wrapText pass this way.
//
// What is left is a word wrapper of its own. It fails unless oneWrapperAllowed
// names it with its reason. An allowlisted function that no longer exists, or
// that the rule no longer flags, fails too, so the list only shrinks.
//
// Limits, stated so nobody leans on the guard for more: it reads function
// declarations only (a wrapper written as a func literal in a variable is not
// seen), and it judges each function by its own body (a delegator that calls a
// differently named helper which loops over words is not followed).

// oneWrapperName is the name test of rule 1.
var oneWrapperName = regexp.MustCompile(`(?i)(wrap|splitstring)`)

// oneWrapperShared are the shared wrappers a delegator may call.
var oneWrapperShared = map[string]bool{"WrapAnsi": true, "WrapPrompt": true}

// oneWrapperAllowed are the word wrappers outside internal/messaging that exist
// today, keyed by package directory and function name (receiver type first for
// a method), each with the reason it is not yet a call to messaging. A new one
// needs the owner's yes.
var oneWrapperAllowed = map[string]string{
	"internal/util.SplitString": "A second full word wrapper (regexp words, own width count), " +
		"used in about 30 files and, as SplitStringNL, by the splitstring template function. " +
		"Folding it into messaging.WrapAnsi changes player-visible text across the game and " +
		"needs its own snapshot review: a separate change awaiting the owner " +
		"(plan 2026-11-21-prompt-wrap-unify, Out of scope).",
	"internal/util.SplitStringNL": "Joins util.SplitString's lines with CRLF and a prefix; " +
		"it goes when SplitString is folded into messaging.WrapAnsi (same Out of scope entry).",
	"internal/usercommands.wrapPrompt": "Wraps a raw prompt TEMPLATE for the `set` status screen, " +
		"breaking only at a space or before a `{` so no {token} is split; messaging.WrapPrompt " +
		"wraps rendered text and would break inside a token.",
	"internal/rooms.wrapProse": "Folds authored room YAML prose into lines that YAML folding " +
		"rejoins into exactly the input (byte counts, single spaces only, refuses a word wider " +
		"than the line); a file-format job, not text sent to a player.",
	"internal/rooms.wrapDescriptionParts": "Wraps each description part with util.SplitString " +
		"(not its own word loop), then highlights nouns and colours each plain line; it moves " +
		"to messaging.WrapAnsi when SplitString is folded in.",
	"internal/templates.wrapBreak": "Its loop only splits a word wider than a whole line into " +
		"line-wide pieces (player-written signs and runes); the wrapping itself is wrapText, " +
		"which is messaging.WrapAnsi.",
}

// oneWrapperFunc is one function declaration rule 1 to 3 kept.
type oneWrapperFunc struct {
	key      string
	pkgDir   string
	decl     *ast.FuncDecl
	msgAlias string // the file's local name for internal/messaging, "" when not imported
}

// oneWrapperKey is the allowlist key of fn in the package at dir.
func oneWrapperKey(dir string, fn *ast.FuncDecl) string {
	name := fn.Name.Name
	if fn.Recv != nil && len(fn.Recv.List) == 1 {
		t := fn.Recv.List[0].Type
		if star, ok := t.(*ast.StarExpr); ok {
			t = star.X
		}
		if idx, ok := t.(*ast.IndexExpr); ok {
			t = idx.X
		}
		if id, ok := t.(*ast.Ident); ok {
			name = id.Name + "." + name
		}
	}
	return dir + "." + name
}

// oneWrapperProducesText is rule 3: a result of type string or []string.
func oneWrapperProducesText(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, r := range fn.Type.Results.List {
		t := r.Type
		if arr, ok := t.(*ast.ArrayType); ok && arr.Len == nil {
			t = arr.Elt
		}
		if id, ok := t.(*ast.Ident); ok && id.Name == "string" {
			return true
		}
	}
	return false
}

// oneWrapperMessagingAlias is the name f uses for internal/messaging.
func oneWrapperMessagingAlias(f *ast.File) string {
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !strings.HasSuffix(path, "internal/messaging") {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "messaging"
	}
	return ""
}

// oneWrapperCandidates applies rules 1 to 3 to one file in dir.
func oneWrapperCandidates(dir string, f *ast.File) []oneWrapperFunc {
	if dir == "internal/messaging" || strings.HasPrefix(dir, "internal/messaging/") {
		return nil
	}
	alias := oneWrapperMessagingAlias(f)
	var out []oneWrapperFunc
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !oneWrapperName.MatchString(fn.Name.Name) || !oneWrapperProducesText(fn) {
			continue
		}
		out = append(out, oneWrapperFunc{key: oneWrapperKey(dir, fn), pkgDir: dir, decl: fn, msgAlias: alias})
	}
	return out
}

// oneWrapperOwnWrappers applies rule 4 to the candidates and returns the keys
// of those that wrap words themselves, sorted. Delegation through a function
// of the same package is resolved to a fixed point.
func oneWrapperOwnWrappers(cands []oneWrapperFunc) []string {
	delegates := map[string]bool{} // by key
	for changed := true; changed; {
		changed = false
		for _, c := range cands {
			if delegates[c.key] {
				continue
			}
			loops, delegatesNow := false, false
			ast.Inspect(c.decl.Body, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.ForStmt, *ast.RangeStmt:
					loops = true
				case *ast.CallExpr:
					switch fun := node.Fun.(type) {
					case *ast.SelectorExpr:
						if x, ok := fun.X.(*ast.Ident); ok && c.msgAlias != "" && x.Name == c.msgAlias && oneWrapperShared[fun.Sel.Name] {
							delegatesNow = true
						}
					case *ast.Ident:
						if delegates[c.pkgDir+"."+fun.Name] {
							delegatesNow = true
						}
					}
				}
				return true
			})
			if delegatesNow && !loops {
				delegates[c.key] = true
				changed = true
			}
		}
	}
	var own []string
	for _, c := range cands {
		if !delegates[c.key] {
			own = append(own, c.key)
		}
	}
	sort.Strings(own)
	return own
}

// TestNoNewWordWrapper is the guard.
func TestNoNewWordWrapper(t *testing.T) {
	_, files, _ := wrapGuardParse(t)
	var cands []oneWrapperFunc
	for _, gf := range files {
		cands = append(cands, oneWrapperCandidates(gf.dir, gf.file)...)
	}
	own := oneWrapperOwnWrappers(cands)
	found := map[string]bool{}
	for _, key := range own {
		found[key] = true
		if _, ok := oneWrapperAllowed[key]; !ok {
			t.Errorf("%s is a word wrapper outside internal/messaging. Call messaging.WrapAnsi or "+
				"messaging.WrapPrompt; a second wrapper needs the owner's yes and an entry in "+
				"oneWrapperAllowed with its reason.", key)
		}
	}
	var stale []string
	for key := range oneWrapperAllowed {
		if !found[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		t.Errorf("stale allowlist entry: %s is no longer a word wrapper of its own (removed, renamed, "+
			"or now a call to messaging). Delete it from oneWrapperAllowed.", key)
	}
}

// TestNoNewWordWrapper_CanFail is the null probe: the rule on fixtures.
func TestNoNewWordWrapper_CanFail(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want []string
	}{
		{"a word loop", `func wrapLines2(s string, w int) []string {
			var out []string
			for _, word := range strings.Fields(s) { out = append(out, word) }
			return out
		}`, []string{"internal/p.wrapLines2"}},
		{"a method that splits", `func (b *Box) SplitStringish(s string) string { for range s {}; return s }`,
			[]string{"internal/p.Box.SplitStringish"}},
		{"a delegator", `func wrapLines(s string, w int) []string { return strings.Split(messaging.WrapAnsi(s, w), "\n") }`, nil},
		{"a delegator through a sibling", `func wrapA(s string) string { return messaging.WrapPrompt(s, 9) }
			func wrapB(s string) string { return wrapA(s) + "!" }`, nil},
		{"a delegator that also loops", `func wrapC(s string) string { for range s {}; return messaging.WrapAnsi(s, 9) }`,
			[]string{"internal/p.wrapC"}},
		{"not text", `func (e *E) Unwrap() error { return nil }
			func wrapCoord(v, n int) int { return v % n }
			func wrapsProcAlone(n *Node) bool { for n != nil {}; return true }`, nil},
		{"another package's WrapAnsi", `func wrapD(s string) string { return other.WrapAnsi(s, 9) }`,
			[]string{"internal/p.wrapD"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package p\nimport (\"strings\"; \"github.com/GoMudEngine/GoMud/internal/messaging\")\n" + tc.src + "\n"
			f, err := parser.ParseFile(token.NewFileSet(), "probe.go", src, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := oneWrapperOwnWrappers(oneWrapperCandidates("internal/p", f))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("own wrappers %q, want %q", got, tc.want)
			}
		})
	}
	t.Run("internal/messaging is the home", func(t *testing.T) {
		f, err := parser.ParseFile(token.NewFileSet(), "probe.go",
			"package messaging\nfunc wrap(s string, w int) string { for range s {}; return s }\n", 0)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := oneWrapperCandidates("internal/messaging", f); len(got) != 0 {
			t.Fatalf("candidates %v in internal/messaging, want none", got)
		}
	})
}
