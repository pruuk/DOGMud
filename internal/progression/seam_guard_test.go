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

	// ── Added by U10b-1 Task 22, when this walk widened past combat+hooks ──
	//
	// The two Actor ADAPTERS. actor_user.go and actor_mob.go implement the
	// Actor interface's OnSkillUse/OnStatUse by delegating to the Character.
	// They ARE the seam's plumbing; flagging them would be flagging the
	// interface for existing.
	filepath.Join("internal", "actions", "actor_user.go"): true,
	filepath.Join("internal", "actions", "actor_mob.go"):  true,

	// The shop MERCHANT's charisma roll on a completed trade. Checked in Task
	// 18c rather than assumed: it belongs to a DIFFERENT character from the
	// one receiving the bartering award -- the merchant, not the buyer or
	// seller -- so it is not the stray-stat-roll-beside-an-award shape Task 22
	// deleted from emitAttackerStatGain. It is the merchant's ONLY progression
	// from trading. Listed so the exemption is a decision on the record.
	filepath.Join("internal", "actions", "buy.go"):  true,
	filepath.Join("internal", "actions", "sell.go"): true,

	// HIDDEN DETECTION, and this one is a genuine OPEN ITEM rather than a
	// permanent exemption.
	//
	// go.go's two hidden-detection sites award Search to the observer who WINS
	// the contest and nothing to the one who loses, which is the firing defect
	// this slice removes everywhere else. They are NOT converted here because
	// the same two sites are the "hidden-detection fix" that the settled U10b
	// decisions assign to U10b-1b BY NAME: the flat 135 threshold never reads
	// the hider's sneak score, so the contest itself is wrong, and rewriting
	// its firing before its resolution would mean touching them twice.
	//
	// ⚠️ REMOVE THIS ROW when U10b-1b converts them. It is the only entry here
	// that is expected to be temporary.
	filepath.Join("internal", "usercommands", "go.go"): true,

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
// ⚠️ AwardResolved / AwardResolvedScaled / AwardDefenceProgression are
// DELIBERATELY ABSENT, and the U10b-1 plan's Task 22 Step 1 was wrong to ask
// for AwardResolved here. They are the SEAM every converted site is supposed
// to route THROUGH; listing them would flag exactly the code this slice spent
// twenty commits producing, and the only way to keep the guard green would be
// to allow-list every converted file, which is the same as deleting the guard.
//
// The map holds RAW primitives: the things that fire progression while
// bypassing the seam.
var progressionCalls = map[string]bool{
	"OnSkillUse": true, "OnSkillUseScaled": true,
	"OnStatUse": true, "OnStatUseScaled": true,
	"CheckSkillProgression": true, "CheckStatProgression": true,
	// OnCriticalSuccess / OnCriticalFailure are NOT listed: U10b-1 Task 21
	// deleted the last nine definitions of them, all test fakes, and the
	// Actor interface dropped them back in U9. Naming a method that exists
	// nowhere makes this map read as though the guard covers more than it
	// does. OnCritReceived STAYS -- it is a real method and the plan is
	// explicit that this task does not touch it.
	"OnCritReceived": true,
	"TrackSkillUse":  true, "TrackStatUse": true,
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
	// U10b-1 Task 22 WIDENED this walk. It covered only internal/combat and
	// internal/hooks, which was right when the arc's subject was the contest
	// paths -- but U10b-1 converted roughly fifty sites across five more
	// packages, and a guard that cannot see them is not the guard the slice's
	// "done when" item 1 asks for ("every rolled site routes through the seam,
	// and a guard test fails a new one that does not").
	//
	// internal/characters is DELIBERATELY ABSENT: it is the applier's own home,
	// so every primitive is defined and legitimately called there.
	totalScanned := 0
	for _, pkg := range []string{
		"internal/combat", "internal/hooks",
		"internal/actions", "internal/usercommands", "internal/mobcommands",
		"internal/mobs",
	} {
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

		// A guard that silently scans zero files passes forever and protects
		// nothing, which is what a path change or a bad filter produces.
		//
		// The floor was 20, calibrated when this walked only combat and hooks.
		// U10b-1 Task 22 widened it to six packages and internal/mobs has 19
		// non-test files, so a flat 20 failed on a package it was scanning
		// perfectly well. Lowered to 5 per package -- still far above zero,
		// which is the failure this actually guards -- and backed by a TOTAL
		// floor, which is what would catch a filter bug that quietly gutted
		// every package at once.
		if scanned < 5 {
			t.Errorf("guard inspected only %d files in %s; it is not actually scanning the package and would pass no matter what the code did",
				scanned, pkg)
		}
		totalScanned += scanned
	}

	if totalScanned < 150 {
		t.Errorf("guard inspected only %d files across all packages; the walk is not reaching the codebase it claims to cover",
			totalScanned)
	}
}
