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

// guardedRollFuncs are the roll entry points a production caller must not reach
// for, keyed by the package they live in.
//
// Two different reasons appear here:
//
//   - dice.OpposedRollStatRaw / dice.OpposedRoll and contest.Run /
//     contest.AgainstDifficulty apply NO floor. That is the original chunk 5.9
//     hazard. internal/contest joined the list in U4: Run and AgainstDifficulty
//     are exported and unfloored, so before U4 a new contest could opt out of
//     the floors through the very package this arc built to prevent that -- and
//     the guard, which hardcoded the callee package to "dice", said nothing.
//   - dice.OpposedRollStat / dice.OpposedRollStatWithFloors ARE floored, and are
//     guarded for a different reason: U4 emptied them of production callers and
//     U6 deletes them. The risk is drift BACK onto the legacy path.
var guardedRollFuncs = map[string]map[string]bool{
	"dice": {
		"OpposedRollStatRaw":        true,
		"OpposedRoll":               true,
		"OpposedRollStat":           true,
		"OpposedRollStatWithFloors": true,
	},
	"contest": {
		"Run":               true,
		"AgainstDifficulty": true,
		"RunWithFloors":     true,
	},
}

// guardedRollExemptions lists the callers allowed to reach each guarded entry
// point, with the reason each one genuinely needs it.
//
// Keyed by callee package, then by a repo-relative FILE or DIRECTORY. A caller
// matches if its path equals a key or sits underneath one.
//
// Prefer a FILE key. internal/combat is 30+ files and is the single most likely
// place a new unfloored contest gets written; a directory exemption there would
// blind the guard in the package it most needs to watch. Exactly one call needs
// it, so exactly one file is named.
var guardedRollExemptions = map[string]map[string]string{
	"dice": {
		// Owns the primitives and delegates between them: OpposedRollStat ->
		// OpposedRollStatWithFloors -> OpposedRollStatRaw -> OpposedRoll.
		"internal/dice": "owns the roll primitives and delegates between them",
	},
	"contest": {
		// Melee is the one floor style that floors AFTER the contest rather than
		// inside the roll: resolveDefenseOutcomeCore floors a computed hit
		// CHANCE, not a roll outcome. Reconciling the two styles is an open U6
		// question; until then this single call is correct.
		"internal/combat/combat_helpers.go": "floors after the contest in resolveDefenseOutcomeCore",
		// Defines combat.RunContest, so it is the one place that legitimately
		// hands contest.RunWithFloors an explicit floor.
		"internal/combat/run_contest.go": "defines combat.RunContest",
		// Defines combat.RunConcentrationContest — the one ConcentrationFloor
		// reader (U10).
		"internal/combat/run_concentration_contest.go": "defines combat.RunConcentrationContest — the one ConcentrationFloor reader (U10)",
		// U10b-1b Phase A. Search's non-stealth tiers are STATIC-DIFFICULTY
		// (roadmap category B), and combat.RunContest's own doc comment reserves
		// itself for opposed contests and says these are deliberately unfloored:
		// "Do not route them here to 'unify' them." AgainstDifficulty is the
		// seam built for them, and it applies no floor by design.
		//
		// The two remaining dice.RollStat sites in this file are the hidden
		// PLAYER and MOB checks. Those are genuinely opposed and go to
		// combat.RunContest in Phase C, not here.
		"internal/actions/search.go": "U10b-1b: static-difficulty search tiers, deliberately unfloored (category B)",
		// U10b-1b Phase A. Track's 125 DETECTION GATE, same category B reasoning.
		// The finer 135/175 quality bands still read the attacker's own roll --
		// see the ruling in track.go for why they are not separate contests.
		"internal/actions/track.go": "U10b-1b: static-difficulty track detection gate, deliberately unfloored (category B)",
	},
}

// isExempt reports whether a caller is covered by an exemption set. It matches a
// FILE key exactly, or a DIRECTORY key against the file's directory or any
// directory beneath it.
//
// A nil or missing map yields false, which is the safe default: an unlisted
// callee package exempts nobody.
func isExempt(rel, dir string, exemptions map[string]string) bool {
	for exempt := range exemptions {
		if rel == exempt || dir == exempt || strings.HasPrefix(dir, exempt+"/") {
			return true
		}
	}
	return false
}

// TestOpposedContestsAreFloored fails when production code reaches for a
// guarded roll entry point without an exemption.
//
// This is the recurrence guard for roadmap chunk 5.10, extended by U4. The
// floors were written for combat, lived in internal/combat/combat_helpers.go,
// and every contest added afterwards silently got the unfloored path --
// stealth, theft, traps, detection, spells and maneuvers. A stat-100 thief
// against a stat-150 mark succeeded 0.9% of the time. Nobody chose that.
//
// The guidance pointed the wrong way too: CLAUDE.md told developers to use
// dice.OpposedRollStat, and before 5.10 that was the UNFLOORED function. Chunk
// 5.10 made the floored roll the default by giving it the natural name; U4 then
// moved every production caller onto the internal/combat wrapper family and
// deprecated the dice pair, so CLAUDE.md was updated again. U6 then collapsed
// that wrapper family to a single entry point. If you are reading this because
// the guard failed, read the docs on combat.RunContest in
// internal/combat/run_contest.go before adding an exemption.
//
// KNOWN BLIND SPOT: the visitor matches only package-qualified calls
// (pkg.Func). A same-package call inside internal/dice or internal/contest is a
// bare identifier and is invisible here. Those two packages own the primitives
// and compose them internally, which is why that is acceptable rather than a
// gap to close.
//
// Test files are deliberately NOT scanned. Tests probe the raw distribution on
// purpose (see internal/combat/regression_test.go, which asserts on z-scores);
// the risk being guarded is a PRODUCTION contest silently opting out.
//
// If you are adding a caller that genuinely applies its own floors -- as
// combat's resolveAttack does, flooring a computed hit CHANCE rather than a roll
// outcome -- add its FILE to the matching guardedRollExemptions entry with a
// reason. If you cannot write the reason, you want a floored wrapper.
func TestOpposedContestsAreFloored(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	fset := token.NewFileSet()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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

		dir := filepath.ToSlash(filepath.Dir(rel))

		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			// A syntax error is the compiler's problem to report, not this
			// test's. Skipping it keeps failures attributable.
			return nil
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
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if !guardedRollFuncs[pkg.Name][sel.Sel.Name] {
				return true
			}
			if isExempt(rel, dir, guardedRollExemptions[pkg.Name]) {
				return true
			}
			offenders = append(offenders,
				rel+": "+pkg.Name+"."+sel.Sel.Name+" at line "+
					strconv.Itoa(fset.Position(call.Pos()).Line))
			return true
		})

		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("guarded opposed rolls outside the exemption list:\n  %s\n\n"+
			"Use combat.RunContest -- it is the single entry point for every "+
			"opposed contest and the one place Balance.ContestFloor is read. If "+
			"this caller genuinely applies its own floors, add its file to the "+
			"matching guardedRollExemptions entry with a reason. If you cannot "+
			"write the reason, you want combat.RunContest.",
			strings.Join(offenders, "\n  "))
	}
}
