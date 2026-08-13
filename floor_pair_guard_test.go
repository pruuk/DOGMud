package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// siteExpectation declares what a migrated contest site must look like.
type siteExpectation struct {
	pair  string   // "global", "maneuver" or "spell"; every call in the file
	pairs []string // set INSTEAD of pair when one file legitimately mixes pairs
	calls int      // how many floor-wrapper calls the file must contain

	attackArgs []string // sorted identifier names appearing as the FIRST argument
}

// floorPairOwners records the floor pair, call count and attacker direction of
// every migrated contest site.
//
// WHY THIS TEST EXISTS. config.yaml ships MinContestSuccessChance,
// MinSpellHitChance and MinManeuverHitChance all at 0.05. So wiring a site to
// the WRONG pair changes nothing observable in production: it passes every unit
// test, every statistical check and every playtest, and turns into a live
// balance bug the moment U6 retunes one pair independently. No behavioural test
// can catch that. This one reads the source instead.
//
// The pairs encode the COST OF A SINGLE FAILURE, which is the principle the 5.9
// arc settled on -- a maneuver burns the whole round, an out-of-combat attempt
// is one shot plus a consequence. The mapping below is a design statement, not
// an implementation detail.
//
// attackArgs additionally pins WHO IS ATTACKING. usercommands/go.go is the
// dangerous one: two of its four contests roll the mover's sneak as the attack
// and two roll the mover's search, and swapping them compiles and inverts
// stealth for the whole game.
var floorPairOwners = map[string]siteExpectation{
	// --- U4: out-of-combat / stealth contests, on the GLOBAL pair. ---
	"internal/actions/sneak.go":  {pair: "global", calls: 2, attackArgs: []string{"sneakScore", "sneakScore"}},
	"internal/actions/shadow.go": {pair: "global", calls: 1, attackArgs: []string{"searchScore"}},
	"internal/actions/steal.go": {pair: "global", calls: 4,
		attackArgs: []string{"attackerScore", "attackerScore", "attackerScore", "searchScore"}},
	"internal/actions/plant.go": {pair: "global", calls: 4,
		attackArgs: []string{"attackerScore", "attackerScore", "attackerScore", "searchScore"}},
	"internal/actions/defuse.go": {pair: "global", calls: 1, attackArgs: []string{"defuseScore"}},
	"internal/usercommands/go.go": {pair: "global", calls: 4,
		attackArgs: []string{"observerScore", "observerScore", "sneakScore", "sneakScore"}},
	"internal/usercommands/skill.skullduggery.shadow.go": {pair: "global", calls: 1, attackArgs: []string{"targetScore"}},

	// Flee is a MANEUVER: contested in combat, costs the round.
	"internal/combat/flee.go": {pair: "maneuver", calls: 2, attackArgs: []string{"fleeScore", "fleeScore"}},

	// --- U3-migrated MANEUVER sites. These predate this guard; the closed check
	// below flagged them as undeclared, and each entry's pair, call count and
	// attacker argument was read from the source on 2026-08-13. ---
	"internal/actions/combat_taunt.go": {pair: "maneuver", calls: 1, attackArgs: []string{"attackScore"}},
	// grapple.go passes result.AttackScore, a selector rather than a plain ident.
	"internal/combat/grapple.go":              {pair: "maneuver", calls: 1, attackArgs: []string{"<non-identifier>"}},
	"internal/combat/skill_moves.go":          {pair: "maneuver", calls: 1, attackArgs: []string{"attackerScore"}},
	"internal/combat/submission.go":           {pair: "maneuver", calls: 1, attackArgs: []string{"atkScore"}},
	"internal/hooks/NewRound_MobRoundTick.go": {pair: "maneuver", calls: 1, attackArgs: []string{"attackScore"}},
	"internal/hooks/Position_GrappleTick.go":  {pair: "maneuver", calls: 1, attackArgs: []string{"ctrlScore"}},
	"internal/usercommands/throw.go":          {pair: "maneuver", calls: 1, attackArgs: []string{"attackerScore"}},

	// --- U3-migrated SPELL sites. ---
	// charm_spell.go passes float64(attackScore), a conversion rather than an ident.
	"internal/hooks/charm_spell.go": {pair: "spell", calls: 1, attackArgs: []string{"<non-identifier>"}},
	"internal/hooks/spell_resolution.go": {pair: "spell", calls: 4,
		attackArgs: []string{"spellAttack", "spellAttack", "spellAttack", "spellAttack"}},

	// --- A file that legitimately MIXES pairs. ---
	// avoidance.go holds both defensive avoidance rolls: TrySpellDeflection is a
	// spell contest and TryStoicResolve is the rhetoric-channel mirror, which
	// rides the maneuver pair. A single `pair` cannot express that, so `pairs`
	// pins the multiset instead. Both calls are still checked: adding a third,
	// removing one, or flipping ONE of them to a different wrapper all fail.
	//
	// KNOWN HOLE, stated rather than papered over: swapping BOTH wrappers at
	// once leaves the multiset identical and passes. Both calls also pass
	// `attackScore` as the first argument, so the direction check has no power
	// here either. Two adjacent avoidance functions are exactly where a
	// copy-paste swap would happen. If U6 touches either, verify by hand that
	// TrySpellDeflection is on the SPELL pair and TryStoicResolve on the
	// MANEUVER pair; keying this map by enclosing function name would close it.
	"internal/combat/avoidance.go": {pairs: []string{"maneuver", "spell"}, calls: 2,
		attackArgs: []string{"attackScore", "attackScore"}},
}

// floorPairFuncs maps a wrapper name to the pair it applies.
var floorPairFuncs = map[string]string{
	"RunWithGlobalFloors":   "global",
	"RunWithManeuverFloors": "maneuver",
	"RunWithSpellFloors":    "spell",
}

// TestMigratedSitesKeepTheirFloorPair checks every declared site for the right
// pair, the right number of contests, and the right attacker.
func TestMigratedSitesKeepTheirFloorPair(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	fset := token.NewFileSet()

	for rel, want := range floorPairOwners {
		path := filepath.Join(root, filepath.FromSlash(rel))

		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			offenders = append(offenders, rel+
				": cannot parse (renamed or deleted? update floorPairOwners): "+perr.Error())
			continue
		}

		var gotArgs []string
		var gotPairs []string
		count := 0
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := wrapperName(call)
			if !ok {
				return true
			}
			gotPair := floorPairFuncs[name]
			count++
			gotPairs = append(gotPairs, gotPair)
			if len(want.pairs) == 0 && gotPair != want.pair {
				offenders = append(offenders, rel+": uses the "+gotPair+
					" pair via "+name+", want the "+want.pair+" pair, at line "+
					strconv.Itoa(fset.Position(call.Pos()).Line))
			}
			if len(call.Args) > 0 {
				if id, ok := call.Args[0].(*ast.Ident); ok {
					gotArgs = append(gotArgs, id.Name)
				} else {
					gotArgs = append(gotArgs, "<non-identifier>")
				}
			}
			return true
		})

		// Mixed-pair files are pinned as a multiset instead of a single pair.
		if len(want.pairs) > 0 {
			sort.Strings(gotPairs)
			wantPairs := append([]string(nil), want.pairs...)
			sort.Strings(wantPairs)
			if strings.Join(gotPairs, ",") != strings.Join(wantPairs, ",") {
				offenders = append(offenders, rel+": floor pairs are ["+
					strings.Join(gotPairs, " ")+"], want ["+strings.Join(wantPairs, " ")+
					"] (this file mixes pairs on purpose; see siteExpectation.pairs)")
			}
		}

		if count != want.calls {
			offenders = append(offenders, rel+": found "+strconv.Itoa(count)+
				" floor-wrapper calls, want "+strconv.Itoa(want.calls)+
				" (a site was added, removed, or left on the legacy path)")
		}

		sort.Strings(gotArgs)
		wantArgs := append([]string(nil), want.attackArgs...)
		sort.Strings(wantArgs)
		if strings.Join(gotArgs, ",") != strings.Join(wantArgs, ",") {
			offenders = append(offenders, rel+": attacker arguments are ["+
				strings.Join(gotArgs, " ")+"], want ["+strings.Join(wantArgs, " ")+
				"] — a contest direction was inverted")
		}
	}

	// Closed, not opt-in: any OTHER production file calling a floor wrapper must
	// be declared. Without this the guard would protect exactly these 8 files
	// forever and silently ignore every new contest.
	offenders = append(offenders, undeclaredWrapperCallers(t, root, fset)...)

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("contest sites on the wrong floor pair or direction:\n  %s\n\n"+
			"config.yaml ships all three pairs at 0.05, so a wrong pair is "+
			"invisible to every behavioural test. See the U4 plan for why the "+
			"pairs differ.",
			strings.Join(offenders, "\n  "))
	}
}

// wrapperName returns the floor-wrapper name a call expression invokes.
// It accepts both combat.RunWithGlobalFloors and the same-package bare form.
func wrapperName(call *ast.CallExpr) (string, bool) {
	var name string
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		name = fn.Sel.Name
	case *ast.Ident:
		name = fn.Name
	default:
		return "", false
	}
	if _, ok := floorPairFuncs[name]; !ok {
		return "", false
	}
	return name, true
}

// undeclaredWrapperCallers finds production files that call a floor wrapper but
// are not declared in floorPairOwners. internal/combat/contest_floors.go is
// skipped because it DEFINES them.
func undeclaredWrapperCallers(t *testing.T, root string, fset *token.FileSet) []string {
	t.Helper()

	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "bin", "_datafiles", "docs", "tools":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "internal/combat/contest_floors.go" {
			return nil // defines the wrappers
		}
		if _, declared := floorPairOwners[rel]; declared {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name, ok := wrapperName(call); ok {
				out = append(out, rel+": calls "+name+" at line "+
					strconv.Itoa(fset.Position(call.Pos()).Line)+
					" but is not declared in floorPairOwners")
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return out
}
