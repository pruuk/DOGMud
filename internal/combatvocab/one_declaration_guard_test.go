package combatvocab

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// A defence name declared as a Go string literal outside this package is
// the drift this package exists to end: three declarations of "dodge" is how
// M4b-2 started. actionspec's cost-action keys share the spelling and are a
// different namespace, so that one file is allowed.
//
// Covered shapes, each anchored so the literal is the whole tail of the
// statement (optionally followed by a trailing "," or ":" and a comment):
//   - assignment/declaration:  X = "dodge"        (X Defence = "dodge", etc.)
//   - a switch arm:            case "dodge":
//   - a function's result:     return "dodge"
//
// Known uncovered, because nothing in the tree does this today: a
// struct-literal field (Foo{Defence: "dodge"}), a map key ("dodge": x), or a
// Defence("dodge") conversion. Widen the regex if one of those appears.
//
// The literal must be the WHOLE right-hand side of the assignment (optionally
// followed by a trailing comment), not just the first value in a multi-value
// assignment: combat's narration helpers write
// `verbYou, verbThey = "dodge", "dodges"` to pick a verb pair, which is not a
// declaration of the name anywhere near the sense this guard polices. The
// looser form `=\s*["`](dodge|...)["`]` (no end anchor) matches that
// assignment too, at internal/combat/combat_helpers.go:1469 and
// internal/combat/surprise_narration.go:61, and is a false positive.
//
// `case` and `return` need a \b in front so "case"/"return" is not matched
// inside a longer identifier (e.g. a hypothetical `notcase "dodge":`). "="
// does NOT get that \b: putting all three branches under one shared \b, as
// in `\b(=|case|return)`, looks tidier but is wrong. "=" is itself a
// non-word rune, and Go source always has a space or tab before it
// ("X = \"dodge\""), so the position immediately before "=" sits between two
// non-word runes (space, "="); \b never fires there, and the assignment shape
// silently stops matching at all. Verified by running the suite: the
// single-\b form scores zero hits against both `DefenceDodge Defence =
// "dodge"` and a planted `const sabotage = "dodge"`. Each branch gets its own
// leading anchor instead.
var defenceLiteral = regexp.MustCompile("(?m)(=\\s*|\\bcase\\s*|\\breturn\\s*)[\"`](dodge|parry|block|quell|defy)[\"`]\\s*(:|,)?\\s*(//.*)?$")

var allowedFiles = map[string]bool{
	"internal/combatvocab/vocab.go": true,
	"internal/actionspec/action.go": true,
}

func repoRoot(t *testing.T) string {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	return filepath.Join(filepath.Dir(here), "..", "..")
}

func scanForDefenceLiterals(t *testing.T, root string) (hits []string, scanned int) {
	for _, base := range []string{"internal", "modules"} {
		err := util.WalkDirTolerant(filepath.Join(root, base), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if allowedFiles[rel] {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, loc := range defenceLiteral.FindAllIndex(raw, -1) {
				hits = append(hits, rel+": "+string(raw[loc[0]:loc[1]]))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return hits, scanned
}

func TestDefenceNamesAreDeclaredOnce(t *testing.T) {
	hits, scanned := scanForDefenceLiterals(t, repoRoot(t))
	if scanned < 500 {
		t.Fatalf("scanned only %d files; the walk is not looking at the repo", scanned)
	}
	for _, h := range hits {
		t.Errorf("defence name declared outside combatvocab: %s", h)
	}
}

// The guard must be able to fail. Copies of the three real shapes it
// covers, planted in a temp tree, must all be found.
func TestDefenceNamesGuardIsNotVacuous(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "somewhere")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	planted := "package somewhere\n\n" +
		"const DefenseDodge string = \"dodge\"\n\n" +
		"func g(x string) {\n" +
		"\tswitch x {\n" +
		"\tcase \"parry\":\n" +
		"\t}\n" +
		"}\n\n" +
		"func f() string {\n" +
		"\treturn \"block\"\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	hits, _ := scanForDefenceLiterals(t, root)
	if len(hits) != 3 {
		t.Fatalf("planted declarations not all found: hits = %v", hits)
	}
}
