package actions

// ambush_parity_guard_test.go — U10d Task 16, guard 2.
//
// THREE production paths open an engagement, and each of them decides what
// Aggro type the engagement carries:
//
//   internal/usercommands/attack.go:Attack        (player -> mob, player -> player)
//   internal/mobcommands/attack.go:Attack         (mob -> player, mob -> mob)
//   internal/behaviortree/actions_combat.go:actAttack  (behaviour-tree mob)
//
// They MUST agree, because downstream combat keys off Aggro.Type: a
// SurpriseAttack engagement makes the opening strike of the round crit on a
// won contest, and it costs the shared "special-move" cooldown.
//
// This guards a bug that was real, not hypothetical. The behaviortree path set
// characters.SurpriseAttack STRAIGHT FROM IsHidden(), bypassing the cooldown
// gate the other two respected — so a btree mob could ambush every round for
// free while a player and a mobcommands mob paid for it. The fix routed it
// through actions.EngageAggroType, which is the ONE place the hidden check and
// the cooldown claim live together (see EngageAggroType in combat_attack.go,
// and the behavioural contract pinned in engage_aggro_type_test.go).
//
// The assertion is structural rather than behavioural, on purpose. Driving the
// behaviortree path end to end needs a loaded room, a mob instance registry and
// a live event queue; the regression it would catch is a one-line assignment.
// An AST guard catches that line directly, and this repo already uses the idiom
// (TestSpecialMoveAdmissionOrdering / TestFireAdmissionOrdering, both in this
// package, via exactCallPositions in command_readiness_drift_test.go).
//
// What it proves:
//
//  1. each path calls actions.EngageAggroType the expected number of times;
//  2. EVERY SetAggro call in each path receives an aggro type that was assigned
//     from an EngageAggroType call in that same function — so the cooldown gate
//     cannot be routed around;
//  3. none of the three files mentions characters.SurpriseAttack at all, which
//     is the literal shape of the deleted bug.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ambushEngagementPath names one engagement site and what it must look like.
type ambushEngagementPath struct {
	// file is relative to internal/.
	file string
	// function is the enclosing top-level func.
	function string
	// engageCalls is how many actions.EngageAggroType calls the function makes
	// (one per target kind it can engage).
	engageCalls int
	// setAggroCalls is how many SetAggro calls the function makes. Every one of
	// them is checked against rule 2 above.
	setAggroCalls int
	// why records the target kinds, so a count change has to be justified
	// rather than merely re-numbered.
	why string
}

var ambushEngagementPaths = []ambushEngagementPath{
	{"usercommands/attack.go", "Attack", 2, 2, "player -> mob and player -> player"},
	{"mobcommands/attack.go", "Attack", 2, 2, "mob -> player and mob -> mob"},
	{"behaviortree/actions_combat.go", "actAttack", 1, 1, "btree mob -> whatever the event names"},
}

// internalDirForGuard returns the absolute path of internal/, derived from this
// test file rather than the working directory: test binaries in this package do
// NOT reliably run with the package dir as cwd (economy_test.go chdirs to the
// repo root and all tests in the package share one binary, so relative paths
// resolve differently depending on test ORDER).
func internalDirForGuard(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must resolve this test file")
	return filepath.Dir(filepath.Dir(thisFile))
}

// findTopLevelFunc returns the named top-level func's body, or fails.
func findTopLevelFunc(t *testing.T, file *ast.File, name string) *ast.BlockStmt {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name && fn.Body != nil {
			return fn.Body
		}
	}
	t.Fatalf("func %s not found — the guard's path table has drifted and it is protecting nothing", name)
	return nil
}

// aggroVarsFromEngage collects the names assigned from an
// actions.EngageAggroType call anywhere in the body.
func aggroVarsFromEngage(t *testing.T, fset *token.FileSet, body *ast.BlockStmt) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || formattedASTNode(t, fset, call.Fun) != "actions.EngageAggroType" {
			return true
		}
		for _, lhs := range assign.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok {
				names[ident.Name] = true
			}
		}
		return true
	})
	return names
}

// setAggroCallsIn returns every ENGAGEMENT call in the body, in source order.
//
// An engagement is either the legacy characters.SetAggro or the U12a seam's
// targeting.Commit. Both are accepted because U12a migrates call sites one
// slice at a time: behaviortree and the melee/taunt paths are on Commit, while
// the ~86 sites U12b sweeps still call SetAggro. Recognising both is what lets
// this guard keep watching every path throughout the migration instead of
// going quiet on whichever half moved first.
//
// It deliberately does NOT accept any other name. A path that invents a third
// way to engage is what this guard exists to catch.
func setAggroCallsIn(t *testing.T, fset *token.FileSet, body *ast.BlockStmt) []*ast.CallExpr {
	t.Helper()
	calls := []*ast.CallExpr{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "SetAggro" {
			calls = append(calls, call)
			return true
		}
		// targeting.Commit, but not CommitTaunt/CommitAfter, which are their
		// own verbs and are not part of the ambush path.
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "targeting" && sel.Sel.Name == "Commit" {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

// identsIn returns every identifier appearing anywhere inside expr.
//
// The aggro-type argument used to be a bare variable. Through the seam it is
// targeting.ReasonForAggroType(aggroType), so the check walks the expression
// rather than requiring the argument to BE the identifier. What matters is
// unchanged: the value handed to the engagement call must trace back to
// actions.EngageAggroType and not be conjured locally.
func identsIn(expr ast.Expr) []string {
	names := []string{}
	ast.Inspect(expr, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			names = append(names, id.Name)
		}
		return true
	})
	return names
}

func TestAmbushParityAcrossEngagementPaths(t *testing.T) {
	internalDir := internalDirForGuard(t)

	for _, path := range ambushEngagementPaths {
		t.Run(path.file+":"+path.function, func(t *testing.T) {
			fset := token.NewFileSet()
			// Mode 0 drops comments, so the SurpriseAttack ban below sees only
			// real references — the behaviortree file legitimately names the
			// deleted bug in its own explanatory comment.
			parsed, err := parser.ParseFile(fset,
				filepath.Join(internalDir, filepath.FromSlash(path.file)), nil, 0)
			require.NoError(t, err, "the guard must be able to parse %s", path.file)

			body := findTopLevelFunc(t, parsed, path.function)

			// ── 1. the path routes through the seam ──────────────────────
			engage := exactCallPositions(t, fset, body, "actions.EngageAggroType", true)
			require.Len(t, engage, path.engageCalls,
				"%s:%s must call actions.EngageAggroType once per target kind (%s); "+
					"a missing call means that target kind derives its own aggro type again",
				path.file, path.function, path.why)

			// ── 2. nothing reaches SetAggro around the seam ──────────────
			fromEngage := aggroVarsFromEngage(t, fset, body)
			require.NotEmpty(t, fromEngage,
				"%s:%s assigns nothing from EngageAggroType; the guard cannot see the wiring",
				path.file, path.function)

			setAggro := setAggroCallsIn(t, fset, body)
			require.Len(t, setAggro, path.setAggroCalls,
				"%s:%s makes %d SetAggro calls, expected %d (%s) — a new engagement site "+
					"has appeared and must be checked here too",
				path.file, path.function, len(setAggro), path.setAggroCalls, path.why)

			for _, call := range setAggro {
				require.NotEmpty(t, call.Args, "engagement call made with no arguments")
				last := call.Args[len(call.Args)-1]

				traced := false
				for _, name := range identsIn(last) {
					if fromEngage[name] {
						traced = true
						break
					}
				}
				if !traced {
					t.Fatalf("%s:%s at %s passes %q as the aggro type, and nothing in that "+
						"expression is assigned from actions.EngageAggroType. Every engagement "+
						"path must type itself through that seam: it is the ONE place the hidden "+
						"check and the shared special-move cooldown claim live together. Deriving "+
						"the type locally is how the behaviortree path came to ambush on a "+
						"cooldown it never paid.",
						path.file, path.function, fset.Position(call.Pos()),
						formattedASTNode(t, fset, last))
				}
			}

			// ── 3. the literal shape of the deleted bug ──────────────────
			ast.Inspect(parsed, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "SurpriseAttack" {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if ok && pkg.Name == "characters" {
					t.Errorf("%s at %s references characters.SurpriseAttack directly. Only "+
						"actions.EngageAggroType may produce it — deriving it here (from IsHidden(), "+
						"say) skips the special-move cooldown the other engagement paths pay.",
						path.file, fset.Position(sel.Pos()))
				}
				return true
			})
		})
	}
}

// TestAmbushParityGuardIsNotVacuous proves the guard is reading the files it
// claims to. A path table that has drifted to non-existent files or funcs would
// otherwise fail loudly (findTopLevelFunc t.Fatals), but a table that silently
// SHRANK — someone deleting a row instead of fixing a path — would not.
func TestAmbushParityGuardIsNotVacuous(t *testing.T) {
	require.Len(t, ambushEngagementPaths, 3,
		"there are exactly three engagement paths (player command, mob command, behaviour tree); "+
			"if a fourth has appeared it must be added here, and if one was deleted say so in this message")

	internalDir := internalDirForGuard(t)
	for _, path := range ambushEngagementPaths {
		full := filepath.Join(internalDir, filepath.FromSlash(path.file))
		require.True(t, strings.HasSuffix(filepath.ToSlash(full), path.file),
			"path resolution is broken for %s", path.file)
	}
}

// TestTargetingSeamCoversTheProofSet asserts that every file U12a migrated
// commits through internal/targeting rather than writing aggro itself.
//
// The player and behaviour-tree paths drifted before: U10d had to route the
// btree ambush through EngageAggroType after it had been setting
// SurpriseAttack straight from IsHidden(). A divergence here is invisible at
// runtime, so it is pinned at the source level.
//
// These five files are U12a's proof set. The ~86 sites U12b sweeps are
// deliberately NOT listed: they still call SetAggro, and that is the correct
// state until that slice runs.
func TestTargetingSeamCoversTheProofSet(t *testing.T) {
	internalDir := internalDirForGuard(t)

	proofSet := []string{
		"actions/melee_target.go",
		"actions/combat_taunt.go",
		"behaviortree/actions_combat.go",
		"hooks/pinnacle_tick.go",
		"characters/taunt_hold.go",
	}

	for _, rel := range proofSet {
		t.Run(rel, func(t *testing.T) {
			path := filepath.Join(internalDir, filepath.FromSlash(rel))
			// Mode 0 drops comments so an explanatory mention of the old API
			// in prose does not trip the check.
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, path, nil, 0)
			require.NoError(t, err, "the guard must be able to parse %s", rel)

			ast.Inspect(parsed, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "SetAggro", "EndAggro":
					t.Errorf("%s at %s calls %s directly. Every file in the U12a "+
						"proof set must go through internal/targeting: Commit, "+
						"CommitTaunt or Release.",
						rel, fset.Position(sel.Pos()), sel.Sel.Name)
				case "ForceTauntAggro":
					t.Errorf("%s at %s calls ForceTauntAggro, which U12a deleted. "+
						"Use targeting.CommitTaunt, which sets the hold BEFORE "+
						"committing so the taunt's own set passes the hold gate.",
						rel, fset.Position(sel.Pos()))
				}
				return true
			})
		})
	}
}

// TestTargetingSeamGuardIsNotVacuous proves the guard above reads real files
// and would actually fire. A proof set pointing at missing files would pass
// silently and protect nothing.
func TestTargetingSeamGuardIsNotVacuous(t *testing.T) {
	internalDir := internalDirForGuard(t)

	for _, rel := range []string{
		"actions/melee_target.go",
		"actions/combat_taunt.go",
		"behaviortree/actions_combat.go",
		"hooks/pinnacle_tick.go",
		"characters/taunt_hold.go",
	} {
		path := filepath.Join(internalDir, filepath.FromSlash(rel))
		_, err := os.Stat(path)
		require.NoError(t, err, "proof-set file %s must exist for the guard to mean anything", rel)
	}

	// And the detector must actually detect: a synthetic source containing a
	// banned call has to be caught by the same matcher the guard uses.
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "probe.go",
		"package p\nfunc f(c C) { c.SetAggro(1, 0, 0) }\n", 0)
	require.NoError(t, err)

	found := false
	ast.Inspect(parsed, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "SetAggro" {
			found = true
		}
		return true
	})
	require.True(t, found, "the SetAggro matcher must detect a real call")
}
