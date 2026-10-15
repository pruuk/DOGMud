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

// Messaging M7 (#371): a refusal lives in internal/refusal. A Go string
// literal anywhere else in internal/ or modules/ that carries one of these
// phrases is a copy the helpers replaced. A hit is converted to the helper,
// or, if it is not that refusal, the phrase here is narrowed; never
// allow-listed by file. Each phrase's probe proves the pattern can match.
// Later slices add rows. The prompt rows match a literal that opens with a
// question: "<Verb> what|whom|who|how much?", "... which <noun>?" and
// "What would you like to <verb>?". Literals passed straight to
// cmdPrompt.Ask( are skipped by refusalPhraseHits: interactive questions wait
// for the player's answer and are not refusals.
var refusalPhrases = []struct {
	family string
	re     *regexp.Regexp
	probe  string
}{
	{"not here", regexp.MustCompile(`You don't see (them|that|those [a-z]+) here`), "You don't see those remains here."},
	{"not here, echoed", regexp.MustCompile(`You don't see ['"]?%s['"]? here`), `You don't see "%s" here.`},
	{"not here, floor", regexp.MustCompile(`You don't see (any|a|an) "?%s"? (to pick up|to loot|around)`), "You don't see a %s around."},
	{"not here, verb", regexp.MustCompile(`Can't find that to `), "Can't find that to pet."},
	{"not carrying", regexp.MustCompile(`You don't have (a|an|any) "?%s"? (to |that can )`), `You don't have a "%s" to drink.`},
	{"not carrying, bare", regexp.MustCompile(`You don't have (that|that item)\.`), "You don't have that item."},
	{"not carrying, inventory", regexp.MustCompile(`You don't have "%s" in your inventory`), `You don't have "%s" in your inventory.`},
	{"not carrying, seem", regexp.MustCompile(`You don't seem to be carrying`), "You don't seem to be carrying that."},
	{"no target", regexp.MustCompile(`You have no target`), "You have no target!"},
	{"cooldown", regexp.MustCompile(`moment to recover before`), "You need a moment to recover before attempting another special move."},
	{"guild", regexp.MustCompile(`You are not in a guild`), "You are not in a guild."},
	{"companion", regexp.MustCompile(`no companion travelling with you`), "You have no companion travelling with you."},
	{"party leader", regexp.MustCompile(`not the leader of your party|Only the party leader can`), "You are not the leader of your party."},
	{"in combat", regexp.MustCompile(`can't do that while in combat`), "You can't do that while in combat!"},
	{"failure", regexp.MustCompile(`Something went wrong`), "Something went wrong."},
	{"in combat, generic gate", regexp.MustCompile(`can't do that while fighting`), "You can't do that while fighting!"},
	{"no target, gone", regexp.MustCompile(`Your target is gone`), "Your target is gone!"},
	{"cooldown, moment", regexp.MustCompile(`You need a moment before you can`), "You need a moment before you can do that."},
	{"not here, nothing by that name", regexp.MustCompile(`Nothing by that name is in this room`), "Nothing by that name is in this room."},
	{"not here, nobody like that", regexp.MustCompile(`nobody like that here to ask`), "There is nobody like that here to ask."},
	{"companion, reset", regexp.MustCompile(`You have no companion to`), "You have no companion to reset."},
	{"not here, who", regexp.MustCompile(`^Who\?\?\?`), "Who???"},
	{"prompt", regexp.MustCompile(`(?i)^\S+( (on|from|to|at|with|in|into|for|about))? (what|whom|who|how much( gold)?)\?`), "Pet what?"},
	{"prompt, which", regexp.MustCompile(`(?i)^\S+( \S+){0,3} which \w+\?`), "Switch to which target?"},
	{"prompt, would you like", regexp.MustCompile(`(?i)^What (would you like|do you want) to \w+\?`), "What would you like to sell?"},
}

// refusalPhraseHits reports every literal in src that carries a phrase.
func refusalPhraseHits(t *testing.T, rel string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	// Literal arguments of a cmdPrompt.Ask( call are interactive questions.
	asked := map[*ast.BasicLit]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Ask" {
			return true
		}
		if recv, ok := sel.X.(*ast.Ident); ok && recv.Name == "cmdPrompt" {
			for _, a := range call.Args {
				if lit, ok := a.(*ast.BasicLit); ok {
					asked[lit] = true
				}
			}
		}
		return true
	})
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || asked[lit] {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			s = lit.Value
		}
		for _, p := range refusalPhrases {
			if p.re.MatchString(s) {
				hits = append(hits, rel+":"+strconv.Itoa(fset.Position(lit.Pos()).Line)+": "+p.family+": "+s)
			}
		}
		return true
	})
	return hits
}

func TestRefusalPhrasesLiveInThePackage(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(here)
	scanned := 0
	for _, top := range []string{"internal", "modules"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/refusal/") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, h := range refusalPhraseHits(t, rel, src) {
				t.Errorf("%s (use internal/refusal)", h)
			}
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

// TestRefusalPhraseGuardCanFail runs the scanner over a source file holding
// every probe and requires one hit per phrase.
func TestRefusalPhraseGuardCanFail(t *testing.T) {
	var b strings.Builder
	b.WriteString("package probe\n\nvar probes = []string{\n")
	for _, p := range refusalPhrases {
		if !p.re.MatchString(p.probe) {
			t.Errorf("%s: probe %q does not match its own pattern", p.family, p.probe)
		}
		b.WriteString("\t" + strconv.Quote(p.probe) + ",\n")
	}
	b.WriteString("}\n")
	hits := refusalPhraseHits(t, "probe.go", []byte(b.String()))
	joined := strings.Join(hits, "\n")
	for _, p := range refusalPhrases {
		if !strings.Contains(joined, ": "+p.family+": ") {
			t.Errorf("%s: no hit for its probe:\n%s", p.family, joined)
		}
	}
}

// TestRefusalPhraseGuardSkipsOnlyAskQuestions: a literal handed straight to
// cmdPrompt.Ask is an interactive question and is skipped; the same phrase
// anywhere else, even nested inside an Ask call or on another receiver's
// Ask, is still a hit.
func TestRefusalPhraseGuardSkipsOnlyAskQuestions(t *testing.T) {
	src := "package probe\n\nfunc f() {\n" +
		"\tcmdPrompt.Ask(\"Pet what?\", nil)\n" +
		"\tcmdPrompt.Ask(wrap(\"Shout what?\"), nil)\n" +
		"\tother.Ask(\"Say what?\", nil)\n" +
		"\tsend(\"Get what?\")\n" +
		"}\n"
	hits := strings.Join(refusalPhraseHits(t, "probe.go", []byte(src)), "\n")
	if strings.Contains(hits, "Pet what?") {
		t.Errorf("a cmdPrompt.Ask question was flagged:\n%s", hits)
	}
	for _, want := range []string{"Shout what?", "Say what?", "Get what?"} {
		if !strings.Contains(hits, want) {
			t.Errorf("%q should be a hit:\n%s", want, hits)
		}
	}
}
