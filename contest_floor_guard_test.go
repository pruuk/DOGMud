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

// unflooredRollExemptions lists the packages allowed to call the UNFLOORED
// opposed rolls directly, with the reason each one genuinely needs them.
//
// Keys are repo-relative directories. A file is exempt if its directory matches
// a key or sits underneath one.
var unflooredRollExemptions = map[string]string{
	// Owns both primitives and delegates between them: OpposedRollStat is
	// implemented in terms of OpposedRollStatRaw, which is implemented in terms
	// of OpposedRoll.
	"internal/dice": "owns the roll primitives and delegates between them",
}

// unflooredRollFuncs are the roll functions that apply no contest floor.
var unflooredRollFuncs = map[string]bool{
	"OpposedRollStatRaw": true,
	"OpposedRoll":        true,
}

// TestOpposedContestsAreFloored fails when a package outside the exemption list
// calls an unfloored opposed roll.
//
// This is the recurrence guard for roadmap chunk 5.10. The floors were written
// for combat, lived in internal/combat/combat_helpers.go, and every contest
// added afterwards silently got the unfloored path -- stealth, theft, traps,
// detection, spells and maneuvers. A stat-100 thief against a stat-150 mark
// succeeded 0.9% of the time. Nobody chose that; it was inherited by whichever
// function the author copied from.
//
// The guidance pointed the wrong way too: CLAUDE.md told developers to use
// dice.OpposedRollStat, and before 5.10 that was the UNFLOORED function. So did
// its own docstring. Chunk 5.10 made the floored roll the default by giving it
// the natural name, which removes the trap for anyone writing ordinary code.
//
// This test covers what the rename cannot: OpposedRollStatRaw and OpposedRoll
// still exist and still work, and a future contest can still reach for one.
//
// Test files are deliberately NOT scanned. Tests probe the raw distribution on
// purpose (see internal/combat/regression_test.go, which asserts on z-scores);
// the risk being guarded is a PRODUCTION contest silently opting out.
//
// If you are adding a caller that genuinely applies its own floors -- as
// combat's resolveAttack does, flooring a computed hit CHANCE rather than a roll
// outcome -- add its directory here with a reason. If you cannot write the
// reason, you want OpposedRollStat.
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
		for exempt := range unflooredRollExemptions {
			if dir == exempt || strings.HasPrefix(dir, exempt+"/") {
				return nil
			}
		}

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
			if !ok || !unflooredRollFuncs[sel.Sel.Name] {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "dice" {
				return true
			}
			offenders = append(offenders,
				rel+": dice."+sel.Sel.Name+" at line "+
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
		t.Errorf("unfloored opposed rolls outside the exemption list:\n  %s\n\n"+
			"Use dice.OpposedRollStat (floored by default), or add the package to "+
			"unflooredRollExemptions with a reason.",
			strings.Join(offenders, "\n  "))
	}
}
