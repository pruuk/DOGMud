package progression_test

import (
	"go/ast"
	"go/parser"
	"go/token"
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

	// internal/hooks/spell_resolution.go is DELIBERATELY ABSENT, though it
	// currently flags. Two sites (the player-caster and mob-caster magical-
	// crit branches) call target.Character.OnCritReceived("magical", ...)
	// directly instead of building a progression.Outcome{ToughenStat: ...}
	// and routing it through characters.ApplyProgression the way
	// defence_multiplier.go and NewRound_DoCombat_unified.go do for melee.
	// That is a real, unaccounted-for gap in U9's coverage -- spell combat's
	// crit-received willpower toughening bypasses the seam this guard exists
	// to protect. It is a finding to route in a follow-up task, not a file to
	// wave through here. Do NOT add an entry for it to get this test green.
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
// KNOWN FAILURE, left uncleaned on purpose: this currently fails on
// internal/hooks/spell_resolution.go (two OnCritReceived call sites -- see
// the comment above). That is real, unrouted duplication the guard correctly
// caught; it is not silenced here because doing so would be exactly the kind
// of allow-listing this guard exists to prevent. Route spell_resolution.go
// through characters.ApplyProgression to turn this green.
func TestContestPathsFireProgressionOnlyThroughTheApplier(t *testing.T) {
	for _, pkg := range []string{"internal/combat", "internal/hooks"} {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, filepath.Join("..", "..", pkg), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", pkg, err)
		}
		for _, p := range pkgs {
			for path, file := range p.Files {
				rel := filepath.Join(pkg, filepath.Base(path))
				if strings.HasSuffix(path, "_test.go") || allowedDirectProgression[rel] {
					continue
				}
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
		}
	}
}
