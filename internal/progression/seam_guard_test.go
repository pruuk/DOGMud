package progression_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Files in the contest paths that are ALLOWED to fire progression directly.
// Everything else in these packages must go through
// characters.ApplyProgression, so the matrix has one implementation.
//
// The ~93 non-contest call sites elsewhere in the codebase (craft, salvage,
// forage, search, steal, and the rest) are deliberately untouched by U9 and are
// not covered by this guard. Routing them is a later slice's job.
//
// internal/characters/progression.go, where the applier itself lives, is
// DELIBERATELY ABSENT: this guard only walks internal/combat and
// internal/hooks, so listing the applier's own package would be dead weight.
var allowedDirectProgression = map[string]bool{
	// The shared five-defence mapping (AwardDefenceProgression).
	filepath.Join("internal", "combat", "defence_multiplier.go"): true,
	// Melee's unarmed fallback and the quadrant-flavoured stat-gain emitter.
	filepath.Join("internal", "hooks", "NewRound_DoCombat_unified.go"): true,
	filepath.Join("internal", "hooks", "NewRound_DoCombat_helpers.go"): true,
	// Crafting is Category C: deliberately out of U9's scope and out of the
	// contest paths; a later slice routes it. Listed so its exemption is a
	// decision on the record rather than an accident of which files got
	// scanned.
	filepath.Join("internal", "hooks", "NewRound_MobRoundTick.go"):  true,
	filepath.Join("internal", "hooks", "NewRound_UserRoundTick.go"): true,

	// internal/combat/combat_helpers.go is DELIBERATELY ABSENT. An earlier
	// task deleted the per-swing defender roll that used to live there. If
	// this guard flags it, something reintroduced the duplication -- do NOT
	// silence it by adding a row here; investigate the regression instead.

	// internal/hooks/spell_resolution.go is NOT listed here (Task 15b, closed
	// the finding below). It used to flag: the player-caster and mob-caster
	// magical-crit branches called target.Character.OnCritReceived("magical",
	// ...) directly instead of building a progression.Outcome{ToughenStat:
	// ...} and routing it through characters.ApplyProgression the way
	// defence_multiplier.go and NewRound_DoCombat_unified.go do for melee.
	// Both sites (and the parallel internal/actions/combat_taunt.go conviction
	// site, which this guard does not scan but which was routed the same way
	// for consistency) now build a progression.Outcome{ToughenStat: ...,
	// Exceptional: ExcAttackCrit}, take progression.BonusEvents, and apply
	// only the defender side via characters.ApplyProgression. The file needs
	// no allow-list entry because it no longer calls anything in
	// progressionCalls directly.
}

// progressionCalls are the method names that fire progression directly. This
// includes both the raw primitives (OnSkillUse, CheckStatProgression, and so
// on) and the consolidated entry points (OnCritReceived, TrackSkillUse,
// TrackStatUse, CheckRegenProgression) that U9 introduced -- otherwise the
// very calls U9 deleted from the contest paths could return through one of
// the newer names without tripping this guard. TrackSkillUse/TrackStatUse in
// particular matter because a bonus event that tracks without a matching
// applier is a curve-decay bug no rate test would catch.
var progressionCalls = map[string]bool{
	"OnSkillUse": true, "OnSkillUseScaled": true, "OnStatUse": true,
	"CheckSkillProgression": true, "CheckStatProgression": true,
	"OnCritReceived": true, "OnCriticalSuccess": true, "OnCriticalFailure": true,
	"TrackSkillUse": true, "TrackStatUse": true,
	"CheckRegenProgression": true,
}

// TestContestPathsFireProgressionOnlyThroughTheApplier is the recurrence
// guard for the U9 arc. U9 gave progression one shape: internal/progression
// is pure and returns events, and characters.ApplyProgression is the single
// place those events get applied. Earlier U9 tasks routed the contest paths
// (melee and the channel defences) through that seam and deleted the
// progression logic duplicated on both the attack and defence sides. This
// test is what stops the duplication from coming back.
//
// Modelled on the U5b AST pool-mutation guard (pool_mutation_guard_test.go at
// the repo root): walk the guarded packages, parse each non-test file, and
// flag any call whose selector name matches a guarded method -- unless the
// file is on the allow-list, in which case the exemption (and its reason) is
// recorded above rather than left as an accident of which files got scanned.
//
// Matching is on the method NAME via a SelectorExpr, not on the receiver
// type, so an unrelated type with a method of the same name would also trip
// this. That has not happened in this codebase; if it ever does, narrow the
// match (e.g. by receiver type or import) rather than allow-listing the file.
//
// Formerly a KNOWN FAILURE on internal/hooks/spell_resolution.go (two
// OnCritReceived call sites -- see the allow-list comment above for what that
// was and how it was closed, Task 15b). Left as history rather than deleted:
// this guard is meant to catch exactly this shape of regression, and the note
// records what it caught once.
func TestContestPathsFireProgressionOnlyThroughTheApplier(t *testing.T) {
	for _, pkg := range []string{"internal/combat", "internal/hooks"} {
		fset := token.NewFileSet()
		dir := filepath.Join("..", "..", pkg)

		// os.ReadDir + ParseFile rather than parser.ParseDir: the latter is
		// deprecated as of Go 1.25 (SA1019), and the lint gate is
		// only-new-issues, so a new file using it fails CI. We only need each
		// file's syntax tree, not package association, so the deprecation's
		// actual concern (ParseDir ignores build tags when grouping files into
		// packages) does not apply here.
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", pkg, err)
		}

		// Count what we actually inspected. A guard that silently scans zero
		// files passes forever and protects nothing, which is what a path
		// change or a bad filter would produce. Both packages are large; 20 is
		// far below their real size and far above zero.
		scanned := 0

		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			if allowedDirectProgression[filepath.Join(pkg, name)] {
				continue
			}
			file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", filepath.Join(pkg, name), err)
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
				if progressionCalls[sel.Sel.Name] {
					t.Errorf("%s fires %s directly; contest paths must go through characters.ApplyProgression",
						fset.Position(call.Pos()), sel.Sel.Name)
				}
				return true
			})
		}

		if scanned < 20 {
			t.Errorf("guard inspected only %d files in %s; it is not actually scanning the package and would pass no matter what the code did",
				scanned, pkg)
		}
	}
}
