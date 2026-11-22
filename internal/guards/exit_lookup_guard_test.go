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

// exitLookupEntry is how many unfiltered exit lookups one function makes and
// why the unfiltered form is right there.
type exitLookupEntry struct {
	plain int
	why   string
}

const whyExitMob = "a mob's own move: a scripted route keeps its secret exits (#781 ruling 13); an NPC's unscripted choice is filtered where it is made"

// exitLookupRegistry is every non-test call in internal/ and modules/ that
// resolves an exit by name with no knower (Room.FindExitByName,
// actions.FindExit), keyed "path|function". #781, owner ruling 1: a player
// path that names an exit uses Room.FindKnownExitByName or
// actions.FindKnownExit, or a guess at a secret exit gives it away.
var exitLookupRegistry = map[string]exitLookupEntry{
	"internal/actions/go.go|FindExit":                {plain: 1, why: "the unfiltered form itself, for mob moves"},
	"internal/actions/move.go|QuoteMobStep":          {plain: 1, why: whyExitMob},
	"internal/mobcommands/go.go|isMoveAttempt":       {plain: 1, why: whyExitMob},
	"internal/mobcommands/go.go|Go":                  {plain: 1, why: whyExitMob},
	"internal/usercommands/keyring.go|KeyRing":       {plain: 2, why: "names the door a key the player already holds opens"},
	"modules/aicompanion/loot.go|mobCompanionUnlock": {plain: 1, why: "a companion's own unlock; its routes never take a secret exit (worldmap.go)"},
}

// exitLookupCalls adds to found, per "rel|function" of one parsed file, the
// calls that resolve an exit by name with no knower.
func exitLookupCalls(rel string, file *ast.File, found map[string]int) {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		key := rel + "|" + lookupFuncName(fd)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := ""
			switch f := call.Fun.(type) {
			case *ast.SelectorExpr:
				callee = f.Sel.Name
			case *ast.Ident:
				callee = f.Name
			}
			if callee == "FindExitByName" || callee == "FindExit" {
				found[key]++
			}
			return true
		})
	}
}

// exitLookupProblems compares found against exitLookupRegistry.
func exitLookupProblems(found map[string]int) []string {
	var problems []string
	for key, got := range found {
		want, ok := exitLookupRegistry[key]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: not in exitLookupRegistry (%d unfiltered exit lookups)", key, got))
		case got != want.plain:
			problems = append(problems, fmt.Sprintf("%s: %d unfiltered exit lookups, registry says %d", key, got, want.plain))
		}
	}
	for key, want := range exitLookupRegistry {
		if _, ok := found[key]; !ok {
			problems = append(problems, key+": in exitLookupRegistry but no unfiltered exit lookup found there (stale)")
		}
		if want.why == "" {
			problems = append(problems, key+": an unfiltered exit lookup needs a reason")
		}
	}
	sort.Strings(problems)
	return problems
}

// TestEveryPlainExitLookupIsRegistered fails when a function resolves an exit
// by name without a knower and is not registered. If you are here for a new
// player command, call Room.FindKnownExitByName(char, ...) or
// actions.FindKnownExit. A mob path or a tool that must see every exit is
// registered with the reason.
func TestEveryPlainExitLookupIsRegistered(t *testing.T) {
	found := map[string]int{}
	for _, root := range []string{"internal", "modules"} {
		fset := token.NewFileSet()
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			exitLookupCalls(filepath.ToSlash(path), file, found)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s (test must run from the repo root): %v", root, err)
		}
	}
	if len(found) == 0 {
		t.Fatal("no unfiltered exit lookups found at all: the walk is broken, not the code")
	}
	if problems := exitLookupProblems(found); len(problems) > 0 {
		t.Errorf("%d exit lookup problem(s):\n  %s\n\n"+
			"A player path that names an exit must use Room.FindKnownExitByName or "+
			"actions.FindKnownExit, or a guess confirms a secret exit (#781).",
			len(problems), strings.Join(problems, "\n  "))
	}
}

// The scanner sees an unfiltered lookup in an unregistered function and
// ignores the filtered one.
func TestExitLookupGuardCanFail(t *testing.T) {
	const src = `package probe
func Bad(r *rooms.Room) { r.FindExitByName("west") }
func AlsoBad(r *rooms.Room) { actions.FindExit(r, "west") }
func Good(r *rooms.Room, c *characters.Character) { r.FindKnownExitByName(c, "west"); actions.FindKnownExit(r, c, "west") }
`
	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]int{}
	exitLookupCalls("internal/probe/probe.go", file, found)
	if found["internal/probe/probe.go|Bad"] != 1 || found["internal/probe/probe.go|AlsoBad"] != 1 {
		t.Fatalf("the scanner missed an unfiltered lookup: %v", found)
	}
	if _, ok := found["internal/probe/probe.go|Good"]; ok {
		t.Fatalf("the scanner counted a filtered lookup: %v", found)
	}
	joined := strings.Join(exitLookupProblems(found), "\n")
	if !strings.Contains(joined, "internal/probe/probe.go|Bad: not in exitLookupRegistry") {
		t.Errorf("an unregistered unfiltered lookup must be reported, got:\n%s", joined)
	}
}
