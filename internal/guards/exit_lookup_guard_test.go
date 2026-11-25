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
	return registryProblems(found, exitLookupRegistry, "unfiltered exit lookups", "exitLookupRegistry")
}

const (
	whyExitResolved = "reads the exit a known lookup (FindKnownExitByName / FindKnownExit) already resolved for this player"
	whyExitAdmin    = "an admin building tool: it edits every exit, secret ones included"
)

// exitReadRegistry is every non-test read of an exit by its name key,
// Room.GetExitInfo(...) or an index into an Exits map (x.Exits[...]), in the
// packages where player verbs live (exitReadDirs), keyed "path|function". A
// read like that answers for a secret exit as for any other, so each one says
// why it cannot confirm an exit the player has not found (#781, ruling 1).
var exitReadRegistry = map[string]exitLookupEntry{
	"internal/actions/defuse.go|resolveDefuseLockTarget":                  {plain: 1, why: whyExitResolved},
	"internal/actions/flee.go|fleeExit":                                   {plain: 1, why: "a mob's own preferred flee exit; a player's flee passes none (usercommands/flee.go)"},
	"internal/actions/follow_secret.go|LearnSecretExitFollowing":          {plain: 1, why: "asks whether the exit the leader took is secret, to teach it"},
	"internal/actions/look.go|ResolveLook":                                {plain: 2, why: whyExitResolved},
	"internal/actions/look.go|ShotExitClosed":                             {plain: 1, why: "the exit ExecuteFire / resolveShootTarget resolved with FindKnownExitByName"},
	"internal/actions/scan.go|Scan":                                       {plain: 1, why: "the lock of an exit Scan already took only when it is not secret"},
	"internal/usercommands/admin.build.go|Build":                          {plain: 1, why: whyExitAdmin},
	"internal/usercommands/admin.room.dispatcher.go|adminRoom_Exit":       {plain: 3, why: whyExitAdmin},
	"internal/usercommands/admin.room.dispatcher.go|adminRoom_SecretExit": {plain: 3, why: whyExitAdmin},
	"internal/usercommands/admin.room.exits.go|room_Edit_Exits":           {plain: 3, why: whyExitAdmin},
	"internal/usercommands/go.go|Go":                                      {plain: 2, why: "the exit FindKnownExit resolved, and the far side of the exit the player just walked through (FindExitTo), unlocked behind them"},
	"internal/usercommands/keyring.go|KeyRing":                            {plain: 1, why: "names the door a key the player already holds opens"},
	"internal/usercommands/lock.go|Lock":                                  {plain: 1, why: whyExitResolved},
	"internal/usercommands/picklock.go|Picklock":                          {plain: 1, why: whyExitResolved},
	"internal/usercommands/unlock.go|Unlock":                              {plain: 1, why: whyExitResolved},
}

// exitReadDirs are the packages exitReadRegistry covers: player verbs and
// the parser that resolves their words.
var exitReadDirs = []string{"internal/usercommands/", "internal/actions/", "internal/parser/"}

// exitReadCalls adds to found, per "rel|function" of one parsed file, the
// reads of an exit by its name key: GetExitInfo calls and Exits[...] indexes.
func exitReadCalls(rel string, file *ast.File, found map[string]int) {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		key := rel + "|" + lookupFuncName(fd)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "GetExitInfo" {
					found[key]++
				}
			case *ast.IndexExpr:
				if sel, ok := x.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Exits" {
					found[key]++
				}
			}
			return true
		})
	}
}

// registryProblems compares found against registry; what names the kind of
// call in each line.
func registryProblems(found map[string]int, registry map[string]exitLookupEntry, what, registryName string) []string {
	var problems []string
	for key, got := range found {
		want, ok := registry[key]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: not in %s (%d %s)", key, registryName, got, what))
		case got != want.plain:
			problems = append(problems, fmt.Sprintf("%s: %d %s, registry says %d", key, got, what, want.plain))
		}
	}
	for key, want := range registry {
		if _, ok := found[key]; !ok {
			problems = append(problems, fmt.Sprintf("%s: in %s but no %s found there (stale)", key, registryName, what))
		}
		if want.why == "" {
			problems = append(problems, key+": an unfiltered exit read needs a reason")
		}
	}
	sort.Strings(problems)
	return problems
}

// walkGoSources parses every non-test .go file under roots and hands it to
// visit with its slash path. A file that does not parse goes to parseFailed:
// a guard must never pass because it skipped a file.
func walkGoSources(roots []string, visit func(rel string, file *ast.File), parseFailed func(path string, err error)) error {
	for _, root := range roots {
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
				parseFailed(path, perr)
				return nil
			}
			visit(filepath.ToSlash(path), file)
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk %s: %w", root, err)
		}
	}
	return nil
}

// TestEveryPlainExitLookupIsRegistered fails when a function resolves an exit
// by name without a knower and is not registered, or, in a player-verb
// package, reads an exit by its key (GetExitInfo, Exits[...]) and is not
// registered. If you are here for a new player command, call
// Room.FindKnownExitByName(char, ...) or actions.FindKnownExit, and read the
// exit only after that. A mob path or a tool that must see every exit is
// registered with the reason.
func TestEveryPlainExitLookupIsRegistered(t *testing.T) {
	found := map[string]int{}
	reads := map[string]int{}
	err := walkGoSources([]string{"internal", "modules"}, func(rel string, file *ast.File) {
		exitLookupCalls(rel, file, found)
		for _, dir := range exitReadDirs {
			if strings.HasPrefix(rel, dir) {
				exitReadCalls(rel, file, reads)
			}
		}
	}, func(path string, err error) {
		t.Errorf("%s does not parse, so the exit lookup guard cannot read it: %v", path, err)
	})
	if err != nil {
		t.Fatalf("%v (test must run from the repo root)", err)
	}
	if len(found) == 0 || len(reads) == 0 {
		t.Fatal("no unfiltered exit lookups or reads found at all: the walk is broken, not the code")
	}
	problems := append(exitLookupProblems(found),
		registryProblems(reads, exitReadRegistry, "exit reads by key", "exitReadRegistry")...)
	if len(problems) > 0 {
		t.Errorf("%d exit lookup problem(s):\n  %s\n\n"+
			"A player path that names an exit must use Room.FindKnownExitByName or "+
			"actions.FindKnownExit, or a guess confirms a secret exit (#781).",
			len(problems), strings.Join(problems, "\n  "))
	}
}

// The read scanner counts GetExitInfo calls and Exits[...] indexes (not
// ExitsTemp, not a range), and the walk reports a file that does not parse.
func TestExitReadGuardCanFail(t *testing.T) {
	const src = `package probe
func Peek(r *rooms.Room) { r.GetExitInfo("west"); _ = r.Exits["west"]; _ = r.ExitsTemp["gap"]; for range r.Exits {} }
`
	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	reads := map[string]int{}
	exitReadCalls("internal/actions/probe.go", file, reads)
	if reads["internal/actions/probe.go|Peek"] != 2 {
		t.Fatalf("the read scanner counted %v, want 2 reads in Peek", reads)
	}
	joined := strings.Join(registryProblems(reads, exitReadRegistry, "exit reads by key", "exitReadRegistry"), "\n")
	if !strings.Contains(joined, "internal/actions/probe.go|Peek: not in exitReadRegistry") {
		t.Errorf("an unregistered exit read must be reported, got:\n%s", joined)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package broken\nfunc {"), 0o600); err != nil {
		t.Fatal(err)
	}
	var failed []string
	err = walkGoSources([]string{dir}, func(string, *ast.File) {}, func(path string, _ error) {
		failed = append(failed, path)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(failed) != 1 {
		t.Errorf("a file that does not parse must be reported, got %v", failed)
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
