package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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

	// --- 3. Construction absolute sets. ---
	// These run in a defined order relative to Validate() and set a pool to an
	// absolute value. Spawn, respawn, companion summon and recovery, revive and
	// admin restore now go through SetPoolFraction, which caps at the reserved
	// ceiling (#285), and need no exemption: do not add them back.
	"internal/characters/character.go": "character construction and full-restore",
	"internal/characters/overrides.go": "ApplyMobOverrides spawn-time set",

	// atFullPools refills Health and Stamina on its own by-value copy of the
	// character so PowerScore rates the build at full pools. The helper lives in
	// its own file so the exemption covers exactly those two writes.
	"internal/combat/powerscore_pools.go": "PowerScore's own by-value copy, refilled so it rates the build at full pools (#301); never written back to any live character",

	// --- 4. Admin commands, which deliberately bypass cost and harm semantics. ---
	"internal/usercommands/admin.zap.go": "admin set-to-1",

	// --- 5. A test fixture living in a non-_test.go file. ---
	// It compiles into the shipping binary, so the walker sees it even though
	// nothing in production calls it.
	"internal/users/test_helpers.go": "test fixture in a non-_test.go file",

	// internal/characters/resources.go is DELIBERATELY ABSENT. It was exempt
	// while it held Heal, which capped at the raw HealthMax and so filled a
	// reserved pool past its ceiling. #285 retired Heal: every caller passed a
	// positive amount and now calls ApplyRestore(PoolHealth, ...).

	// DELIBERATELY ABSENT, though the U5b-1 plan listed both as U5b-2 exemptions:
	// internal/combat/combat_helpers.go and internal/usercommands/flee.go.
	// Neither ever WROTE a pool: combat_helpers.go held the read
	// `targetChar.Stamina < cost` (U5b-2 deleted that gate outright) and flee.go
	// called DeductStamina (U5b-2 deleted that helper and routed the site onto
	// ApplyCostPartial). An exemption neither of them needs would blind this
	// guard on two files for nothing, and combat_helpers.go is the single file
	// in the hottest package this guard most needs to watch. Both now call
	// primitives, which need no exemption at all -- do not add them back.
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
	root := repoRoot(t)
	var err error

	var offenders []string
	fset := token.NewFileSet()
	scanned := 0

	for _, sub := range poolWriteRoots {
		walkRoot := filepath.Join(root, sub)
		err = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					// A test elsewhere can create and remove a temp file under
					// the tree while packages test in parallel; a vanished
					// entry has nothing to scan.
					return nil
				}
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
			scanned++

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
	// A walk that parsed nothing would report no offenders.
	if scanned < 500 {
		t.Fatalf("parsed only %d production Go files under %v; the walk is not seeing the tree, so a pass proves nothing", scanned, poolWriteRoots)
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
