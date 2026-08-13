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

// guardedPoolFields are the three resource pools that must be mutated through
// the U5a primitives -- ApplyCost, ApplyCostPartial, ApplyHarm and ApplyRestore
// -- rather than written directly.
//
// Matching is on the EXACT selector name, so HealthMax, StaminaMax and
// ConvictionMax are not guarded and need no special case: a write to
// c.HealthMax.Value ends in .Value, and a write to c.HealthMax ends in
// .HealthMax. Neither is in this set.
var guardedPoolFields = map[string]bool{
	"Health":     true,
	"Stamina":    true,
	"Conviction": true,
}

// poolWriteExemptions lists the production files allowed to write a pool
// directly, with the reason each one genuinely needs it.
//
// Keyed by a repo-relative FILE or DIRECTORY path; a file matches if its path
// equals a key or sits underneath one. Prefer a FILE key -- a directory
// exemption blinds the guard for every file added to that package afterwards.
//
// Five permanent classes, plus a temporary block that U5b-2 empties as it routes
// each site.
var poolWriteExemptions = map[string]string{
	// --- 1. The primitives themselves. ---
	// setPool is the single sanctioned writer; routing it through itself would
	// be circular.
	"internal/characters/pools.go": "defines the pool primitives",

	// --- 2. THE CLAMP LAYER. ---
	// validatePoolClamps, the reservation clamp and the enchant-withdrawal
	// shrink all write pools directly to enforce the invariants the primitives
	// depend on. These are what the primitives are built ON, not callers of it.
	"internal/characters/validate.go": "implements the pool clamps themselves",

	// --- 3. Construction and spawn-time absolute sets. ---
	// These run in a defined order relative to Validate() and set a pool to an
	// absolute value. No primitive expresses that: cost, harm and restore are
	// all relative deltas.
	"internal/characters/character.go":         "character construction and full-restore",
	"internal/characters/overrides.go":         "ApplyMobOverrides spawn-time set",
	"internal/mobs/mobs.go":                    "mob spawn",
	"internal/hooks/PlayerSpawn_HandleJoin.go": "companion re-summon",
	// Respawn sets pools to a FRACTION of max, which is a reduction for anyone
	// above it and a restore for anyone below. Neither primitive fits.
	"internal/hooks/Life_Cascades.go": "respawn sets pools to a fraction of max",

	// --- 4. Admin commands, which deliberately bypass cost and harm semantics. ---
	"internal/usercommands/admin.zap.go": "admin set-to-1",
	"internal/usercommands/admin.mob.go": "admin mob heal",
	"internal/usercommands/admin.paz.go": "admin restore",
	"internal/mobcommands/suicide.go":    "ReviveOnDeath full restore",
	"internal/usercommands/suicide.go":   "ReviveOnDeath full restore",

	// --- 5. A test fixture living in a non-_test.go file. ---
	// It compiles into the shipping binary, so the walker sees it even though
	// nothing in production calls it.
	"internal/users/test_helpers.go": "test fixture in a non-_test.go file",

	// --- Temporary. U5b-2 and U5c route each of these and delete the entry. ---
	// Each is a real gameplay change, deliberately held out of U5b-1 so that
	// chunk could merge as provably behaviour-neutral.
	//
	// resources.go holds the two deprecated helpers U5b-1 could NOT route,
	// documented in the plan's prose but missing from its exemption map:
	//
	//   - DeductDefenseStamina is full-or-refuse and becomes ApplyCostPartial in
	//     U5b-2. That flips defence from "cannot afford, so no defence" to "pay
	//     what you have", which is a live combat change.
	//   - Heal is a HARM path: buffs.ComputeTickAmount returns a negative value
	//     for TickPercent < 0, and ApplyRestore no-ops on non-positive input, so
	//     wrapping Heal over it would silently delete every health
	//     damage-over-time buff. U5b-1 removed its two signed callers; U5c
	//     retires the function.
	//
	// This is a FILE exemption on the file where both follow-ups land. Delete it
	// once both functions are gone rather than letting it outlive them.
	"internal/characters/resources.go":     "U5b-2 routes DeductDefenseStamina; U5c retires Heal",
	"internal/usercommands/stand.go":       "U5b-2: stand's two-knob gate",
	"internal/mobcommands/cast.go":         "U5b-2: mob cast gains a guard",
	"internal/actions/mutation_helpers.go": "U5b-2: cooldown-rollback ordering",
	// The seven retained health floors. U5b-1 routed the writes themselves onto
	// ApplyHarm but kept the floor at each site, marked NOTE(U5b-2). Removing
	// them is observable -- GMCP ships Character.Health raw and the prompt
	// prints it raw -- so it is a playtested change, not a refactor.
	"internal/hooks/NewRound_AutoHeal.go":     "U5b-2: mob DoT health floors",
	"internal/actions/surprise_attack.go":     "U5b-2: retained health floor",
	"internal/combat/skill_moves.go":          "U5b-2: retained health floor",
	"internal/hooks/combat_shared_helpers.go": "U5b-2: retained health floor + fold-upkeep cost",
	"internal/usercommands/throw.go":          "U5b-2: retained health floors",

	// DELIBERATELY ABSENT, though the U5b-1 plan listed both as U5b-2 exemptions:
	// internal/combat/combat_helpers.go and internal/usercommands/flee.go. Both
	// are U5b-2 sites, but neither WRITES a pool -- combat_helpers.go:562 is the
	// read `targetChar.Stamina < cost` and flee.go:41 calls DeductStamina. An
	// exemption neither of them needs would blind this guard on two files for
	// nothing, and combat_helpers.go is the single file in the hottest package
	// this guard most needs to watch. Do not add them back when U5b-2 lands:
	// routing them means calling a primitive, which needs no exemption at all.
}

// poolWriteRoots are the trees this guard walks. modules/ never mutated a pool
// as of U5b-1 and is listed so that it cannot start to without being noticed.
var poolWriteRoots = []string{"internal", "modules"}

// TestPoolMutationGoesThroughThePrimitives fails when production code assigns a
// resource pool directly instead of routing through the U5a primitives.
//
// This is the recurrence guard for the U5 arc. Before it, 107 production sites
// wrote c.Health, c.Stamina or c.Conviction by hand, and each one carried its
// own hand-rolled clamp -- or, more often, did not. The clamps disagreed with
// each other: some floored at zero, some at one, some not at all; some clamped
// the upper bound, some trusted the caller. That is why a fumbled grenade could
// briefly show a negative number in a player's own prompt.
//
// ApplyCost, ApplyCostPartial, ApplyHarm and ApplyRestore now own the bounds,
// the refusal semantics and the source attribution in one place. The value of
// that is entirely in nobody writing the 108th direct assignment.
//
// WHAT IS CAUGHT: every assignment form -- =, :=, -=, +=, *=, and so on -- plus
// ++ and --, whenever the left-hand side is a selector ending in .Health,
// .Stamina or .Conviction.
//
// KNOWN BLIND SPOTS, stated rather than papered over:
//
//   - Composite literals. Character{Health: 5} is a KeyValueExpr, not an
//     assignment, and is not reported. Construction is an exempt class anyway.
//   - Aliasing. p := &c.Health; *p = 0 is invisible. No production code does
//     this, and an AST guard cannot follow pointers.
//   - Bare identifiers. A method inside internal/characters that assigned to a
//     receiverless Health would not match, but Go has no implicit receiver, so
//     this cannot occur.
//
// Test files are deliberately NOT scanned. Tests set pools directly to build
// fixtures on purpose; the risk being guarded is a PRODUCTION site quietly
// opting out of the bounds.
//
// If you are adding a site that genuinely cannot use a primitive -- an absolute
// set at spawn, or a clamp the primitives are built on -- add its FILE to
// poolWriteExemptions with a reason. If you cannot write the reason, you want a
// primitive.
func TestPoolMutationGoesThroughThePrimitives(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	fset := token.NewFileSet()

	for _, sub := range poolWriteRoots {
		walkRoot := filepath.Join(root, sub)
		err = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, err error) error {
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

			if isExempt(rel, dir, poolWriteExemptions) {
				return nil
			}

			file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if perr != nil {
				// A syntax error is the compiler's problem to report, not this
				// test's. Skipping it keeps failures attributable.
				return nil
			}

			report := func(lhs ast.Expr, form string) {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok || !guardedPoolFields[sel.Sel.Name] {
					return
				}
				offenders = append(offenders,
					rel+":"+strconv.Itoa(fset.Position(sel.Pos()).Line)+
						": direct "+form+" to ."+sel.Sel.Name)
			}

			ast.Inspect(file, func(n ast.Node) bool {
				switch stmt := n.(type) {
				case *ast.AssignStmt:
					// Any Tok: =, :=, +=, -=, *=, /=, and the rest.
					for _, lhs := range stmt.Lhs {
						report(lhs, "assignment ("+stmt.Tok.String()+")")
					}
				case *ast.IncDecStmt:
					report(stmt.X, "assignment ("+stmt.Tok.String()+")")
				}
				return true
			})

			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("direct pool writes outside the exemption list:\n  %s\n\n"+
			"Use Character.ApplyCost or ApplyCostPartial to spend a pool, "+
			"ApplyHarm to damage one, and ApplyRestore to refill one. They own "+
			"the bounds, so the hand-rolled clamp that used to sit beside each "+
			"of these writes is no longer needed and should be deleted with it. "+
			"If this site genuinely cannot use a primitive -- an absolute set at "+
			"spawn, or a clamp the primitives are built on -- add its file to "+
			"poolWriteExemptions with a reason. If you cannot write the reason, "+
			"you want a primitive.",
			strings.Join(offenders, "\n  "))
	}
}
