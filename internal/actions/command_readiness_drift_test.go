package actions

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type specialMoveOrderGuard struct {
	file            string
	function        string
	earlyReturns    []specialMoveEarlyReturn
	postCommitCalls []string
	roundAssignment bool
}

type specialMoveEarlyReturn struct {
	condition string
	field     string
}

func formattedASTNode(t *testing.T, fset *token.FileSet, node ast.Node) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, format.Node(&buf, fset, node))
	return buf.String()
}

func compositeReturnHasTrueField(stmt ast.Stmt, field string) bool {
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	lit, ok := ret.Results[0].(*ast.CompositeLit)
	if !ok {
		return false
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, keyOK := kv.Key.(*ast.Ident)
		value, valueOK := kv.Value.(*ast.Ident)
		if keyOK && valueOK && key.Name == field && value.Name == "true" {
			return true
		}
	}
	return false
}

// TestSpecialMoveAdmissionOrdering catches a validity branch, read-only
// cooldown probe, cost admission, or consuming cooldown being moved across the
// required boundary. It walks each function's Go AST, matches exact guard
// expressions and their immediate typed early returns, checks every occurrence
// of the ordering calls, and proves resolution/effect/round/progression nodes
// remain after the consuming cooldown. Selector-name presence cannot satisfy it.
func TestSpecialMoveAdmissionOrdering(t *testing.T) {
	guards := []specialMoveOrderGuard{
		{"combat_bash.go", "ExecuteBash", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"!char.HasShield() && !naturalBash", "NoShield"}, {"!char.HasBodyPart(\"arms\") && !naturalBash", "NoShield"},
		}, []string{"ExecuteSkillMove", "RecordAndWait", "OnSkillUse"}, false},
		{"combat_trip.go", "ExecuteTrip", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!char.HasBodyPart(\"legs\")", "NoTarget"},
			{"target.Char.IsOnFloor()", "TargetOnFloor"},
		}, []string{"ExecuteSkillMove", "RecordAndWait", "OnSkillUse"}, false},
		{"combat_kick.go", "ExecuteKick", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!char.HasBodyPart(\"legs\")", "NoTarget"},
		}, []string{"ExecuteSkillMove", "RecordAndWait", "OnSkillUse"}, false},
		{"combat_grapple.go", "ExecuteGrapple", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!char.HasBodyPart(\"arms\")", "GrappleImmune"}, {"!target.Found", "NoTarget"},
			{"target.Char.IsGrappling()", "TargetGrappling"},
		}, []string{"ExecuteGrappleMove", "RecordAndWait", "OnSkillUse"}, false},
		{"combat_hamstring.go", "ExecuteHamstring", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!char.HasBodyPart(\"legs\")", "NoLegs"},
			{"sp == nil || (sp.NaturalAttack != items.Bite && sp.NaturalAttack != items.Claws) || char.HasBodyPart(\"hands\")", "NotBeast"},
		}, []string{"ExecuteSkillMove", "AddCondition", "RecordSpecialMove", "OnSkillUse"}, true},
		{"combat_rake.go", "ExecuteRake", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsClawed(char)", "NotClawed"},
		}, []string{"ExecuteSkillMove", "AddCondition", "RecordSpecialMove", "OnSkillUse"}, true},
		{"combat_maul.go", "ExecuteMaul", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsFanged(char)", "NotFanged"},
		}, []string{"ExecuteSkillMove", "AddCondition", "RecordSpecialMove", "OnSkillUse"}, true},
		{"combat_pounce.go", "ExecutePounce", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"char.IsGrappling()", "Grappling"},
			{"!combat.SpeciesIsQuadrupedPredator(char)", "NotPredator"},
		}, []string{"ExecuteSkillMove", "RecordSpecialMove", "OnSkillUse"}, true},
		{"combat_gore.go", "ExecuteGore", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsHorned(char)", "NotHorned"},
		}, []string{"ExecuteSkillMove", "RecordSpecialMove", "OnSkillUse"}, true},
		{"combat_drain.go", "ExecuteDrain", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!combat.SpeciesHasLifeDrain(char)", "NotLifeDrainer"},
		}, []string{"ExecuteSkillMove", "AddCondition", "Heal", "RecordSpecialMove", "OnSkillUse"}, true},
		{"combat_throttle.go", "ExecuteThrottle", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsFanged(char)", "NotFanged"},
		}, []string{"ExecuteSkillMove", "AddCondition", "AddBuff", "InterruptTargetCast", "RecordSpecialMove", "OnSkillUse"}, true},
	}

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	actionsDir := filepath.Dir(thisFile)

	for _, guard := range guards {
		t.Run(guard.function, func(t *testing.T) {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, filepath.Join(actionsDir, guard.file), nil, 0)
			require.NoError(t, err)

			calls := map[string][]token.Pos{}
			var body *ast.BlockStmt
			for _, decl := range parsed.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Name.Name == guard.function {
					body = fn.Body
					break
				}
			}
			require.NotNil(t, body, "%s must exist", guard.function)
			ast.Inspect(body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				if name != "" {
					calls[name] = append(calls[name], call.Pos())
				}
				return true
			})

			require.Len(t, calls["CooldownReady"], 1, "%s must have one read-only cooldown probe", guard.function)
			require.Len(t, calls["admitFullCost"], 1, "%s must have one shared cost admission", guard.function)
			require.Len(t, calls["TryCooldown"], 1, "%s must have one consuming cooldown", guard.function)
			ready := calls["CooldownReady"][0]
			admit := calls["admitFullCost"][0]
			consume := calls["TryCooldown"][0]
			require.Less(t, int(ready), int(admit), "CooldownReady must precede admission")
			require.Less(t, int(admit), int(consume), "admission must precede consuming TryCooldown")

			require.Len(t, calls["resolveActionTarget"], 1, "%s must resolve through the staged-target-aware seam", guard.function)
			require.Less(t, int(calls["resolveActionTarget"][0]), int(ready), "target resolution must precede cooldown/admission")
			if guard.function != "ExecuteHamstring" {
				require.Len(t, calls["commitMeleeEngagement"], 1, "%s must commit staged engagement once", guard.function)
				require.Greater(t, int(calls["commitMeleeEngagement"][0]), int(consume),
					"%s must not commit engagement before successful admission/cooldown", guard.function)
			}

			for _, expected := range guard.earlyReturns {
				found := false
				for _, stmt := range body.List {
					ifStmt, ok := stmt.(*ast.IfStmt)
					if !ok || formattedASTNode(t, fset, ifStmt.Cond) != expected.condition || len(ifStmt.Body.List) != 1 {
						continue
					}
					if compositeReturnHasTrueField(ifStmt.Body.List[0], expected.field) {
						found = true
						require.Less(t, int(ifStmt.Pos()), int(ready), "%s guard %s must precede CooldownReady", guard.function, expected.condition)
					}
				}
				require.True(t, found, "%s must have exact early return `if %s { return ...{%s: true} }`", guard.function, expected.condition, expected.field)
			}

			for _, name := range guard.postCommitCalls {
				require.NotEmpty(t, calls[name], "%s must retain %s", guard.function, name)
				for _, pos := range calls[name] {
					require.Greater(t, int(pos), int(consume), "%s call %s must follow consuming TryCooldown", guard.function, name)
					if guard.function != "ExecuteHamstring" {
						require.Greater(t, int(pos), int(calls["commitMeleeEngagement"][0]),
							"%s call %s must follow engagement commit", guard.function, name)
					}
				}
			}
			if guard.roundAssignment {
				roundPositions := []token.Pos{}
				ast.Inspect(body, func(node ast.Node) bool {
					assign, ok := node.(*ast.AssignStmt)
					if !ok {
						return true
					}
					for _, lhs := range assign.Lhs {
						if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "RoundsWaiting" {
							roundPositions = append(roundPositions, assign.Pos())
						}
					}
					return true
				})
				require.NotEmpty(t, roundPositions, "%s must consume a combat round", guard.function)
				for _, pos := range roundPositions {
					require.Greater(t, int(pos), int(consume), "%s round mutation must follow consuming TryCooldown", guard.function)
				}
			}
		})
	}
}

func handledReturn(stmt ast.Stmt) bool {
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 2 {
		return false
	}
	first, firstOK := ret.Results[0].(*ast.Ident)
	second, secondOK := ret.Results[1].(*ast.Ident)
	return firstOK && secondOK && first.Name == "true" && second.Name == "nil"
}

func nodeHasCall(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			found = found || fn.Name == name
		case *ast.SelectorExpr:
			found = found || fn.Sel.Name == name
		}
		return !found
	})
	return found
}

// TestSpecialMoveWrapperAdmission catches a wrapper using selector names
// without owning the exact refusal branch. Player wrappers must stage target
// acquisition, compare result.Cost.Status to characters.CostRefused, render
// shared prose, then immediately return. Mob wrappers make the same comparison
// and immediately return without text.
func TestSpecialMoveWrapperAdmission(t *testing.T) {
	playerWrappers := []string{"bash", "trip", "kick", "grapple", "rake", "maul", "pounce", "gore", "drain", "throttle"}
	mobWrappers := []string{"bash", "charge", "trip", "kick", "grapple", "hamstring", "rake", "maul", "pounce", "gore", "drain", "throttle"}
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	inspectWrapper := func(t *testing.T, dir, file, function, resultName string, player bool) {
		t.Helper()
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, filepath.Join(repoRoot, "internal", dir, file+".go"), nil, 0)
		require.NoError(t, err)
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != function {
				continue
			}
			expectedCondition := resultName + ".Cost.Status == characters.CostRefused"
			var refusal *ast.IfStmt
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				ifStmt, ok := node.(*ast.IfStmt)
				if ok && formattedASTNode(t, fset, ifStmt.Cond) == expectedCondition {
					refusal = ifStmt
					return false
				}
				return true
			})
			require.NotNil(t, refusal, "%s must own exact refusal comparison %s", function, expectedCondition)
			if player {
				require.True(t, nodeHasCall(fn.Body, "StageMeleeTarget"), "%s must stage target acquisition without engagement", function)
				require.False(t, nodeHasCall(fn.Body, "AcquireMeleeTarget"), "%s must not eagerly acquire/engage", function)
				require.Len(t, refusal.Body.List, 2, "%s refusal branch must send shared text then return", function)
				require.True(t, nodeHasCall(refusal.Body.List[0], "SendText"), "%s refusal branch must send player text first", function)
				require.True(t, nodeHasCall(refusal.Body.List[0], "CostRefusalText"), "%s refusal branch must use shared refusal text", function)
				require.True(t, handledReturn(refusal.Body.List[1]), "%s refusal branch must immediately return handled", function)
			} else {
				require.Len(t, refusal.Body.List, 1, "%s mob refusal branch must be silent and immediate", function)
				require.True(t, handledReturn(refusal.Body.List[0]), "%s mob refusal branch must return handled", function)
				require.False(t, nodeHasCall(fn.Body, "CostRefusalText"), "%s mob wrapper must not render private refusal text", function)
			}
			return
		}
		t.Fatalf("%s.%s wrapper not found", dir, function)
	}

	for _, name := range playerWrappers {
		t.Run("player_"+name, func(t *testing.T) {
			function := map[string]string{
				"bash": "Bash", "trip": "Trip", "kick": "Kick", "grapple": "Grapple", "rake": "Rake",
				"maul": "Maul", "pounce": "Pounce", "gore": "Gore", "drain": "Drain", "throttle": "Throttle",
			}[name]
			resultName := "res"
			if name == "bash" {
				resultName = "bashResult"
			}
			inspectWrapper(t, "usercommands", name, function, resultName, true)
		})
	}
	for _, name := range mobWrappers {
		t.Run("mob_"+name, func(t *testing.T) {
			function := map[string]string{
				"bash": "Bash", "charge": "Charge", "trip": "Trip", "kick": "Kick", "grapple": "Grapple",
				"hamstring": "Hamstring", "rake": "Rake", "maul": "Maul", "pounce": "Pounce", "gore": "Gore",
				"drain": "Drain", "throttle": "Throttle",
			}[name]
			resultName := "res"
			if name == "bash" {
				resultName = "bashResult"
			}
			inspectWrapper(t, "mobcommands", name, function, resultName, false)
		})
	}
}

// driftCase describes one point in the (command × gate) matrix.
type driftCase struct {
	name       string
	cmd        string
	mutate     func(*mobs.Mob)
	wantReady  bool
	wantReason string // Execute*-side flag when not ready; ignored when wantReady=true
}

func setDriftAggroMobTarget(m *mobs.Mob, targetID int) {
	target := &mobs.Mob{InstanceId: targetID}
	target.Character.Name = "Target"
	target.Character.HealthMax.Value = 1_000_000
	target.Character.Health = 1_000_000
	target.Character.StaminaMax.Value = 1_000_000
	target.Character.Stamina = 1_000_000
	setCombatPositionParallel(&target.Character, position.Standing)
	mobs.SetInstanceForTest(targetID, target)
	m.Character.SetAggro(0, targetID, characters.DefaultAttack)
}

func runExecuteAndReadExecuted(cmd string, actor Actor) bool {
	switch cmd {
	case "taunt":
		return ExecuteTaunt(actor).Executed
	case "rally":
		return ExecuteRally(actor).Executed
	case "warcry":
		return ExecuteWarcry(actor).Executed
	case "bash":
		return ExecuteBash(actor).Executed
	case "trip":
		return ExecuteTrip(actor).Executed
	case "kick":
		return ExecuteKick(actor).Executed
	case "grapple":
		return ExecuteGrapple(actor).Executed
	case "hamstring":
		return ExecuteHamstring(actor).Executed
	case "rake":
		return ExecuteRake(actor).Executed
	case "maul":
		return ExecuteMaul(actor).Executed
	case "pounce":
		return ExecutePounce(actor).Executed
	case "gore":
		return ExecuteGore(actor).Executed
	case "drain":
		return ExecuteDrain(actor).Executed
	case "throttle":
		return ExecuteThrottle(actor).Executed
	}
	return false
}

// TestCommandReadinessDrift asserts CommandIsReady and each Execute*
// agree on readiness for a shared actor state. When they diverge, the
// btree's command_best_of can issue a command the command itself
// silently rejects. Surfaced once in T7 smoke testing of the tank_taunter
// archetype (taunt unregistered on mob side); this test is the guard.
//
// To add a new command or gate: add a row below. The helper at the
// bottom of this file dispatches to the right Execute* and reads the
// named result field.
//
// SCOPE LIMIT: This test checks boolean agreement ("is this command
// ready?"), not which specific reason flag Execute* returns. For the
// not-ready path, gate-ordering differences between CommandIsReady
// and Execute* can cause a case to fail for a cryptic reason-mismatch
// even when the boolean agrees; see the bash_cooldown note below for
// a concrete example.
//
// SYNC POINT: when adding a new gate to CommandIsReady or an
// Execute*, add the corresponding drift row here.
func TestCommandReadinessDrift(t *testing.T) {
	cleanup := seedBuffsForTest()
	defer cleanup()

	// Seed a legged species for the trip_ready / kick_ready "happy path" rows.
	// SpeciesId 0 is intentionally left unseeded so the no_legs rows below can
	// rely on the nil-species → no-legs behavior.
	// SpeciesId 7304 is a clawed species for the rake_ready / rake_notclawed rows.
	speciesCleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		7300: {SpeciesId: 7300, Name: "legged-test", BodyParts: []string{"legs"}},
		7304: {SpeciesId: 7304, Name: "clawed-test", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Claws},
		7305: {SpeciesId: 7305, Name: "fanged-test", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
		7306: {SpeciesId: 7306, Name: "horned-test", BodyParts: []string{"legs", "mouth", "horns"}, NaturalAttack: items.Gore},
		7307: {SpeciesId: 7307, Name: "vampire-test", BodyParts: []string{"arms", "hands", "legs", "mouth"}, NaturalAttack: items.Claws, LifeDrain: true},
		7308: {SpeciesId: 7308, Name: "natural-basher-test", BodyParts: []string{"arms", "legs"}, NaturalBash: true},
		7309: {SpeciesId: 7309, Name: "legless-fanged-test", BodyParts: []string{"mouth"}, NaturalAttack: items.Bite},
		// Hands-bearing beast-identity species for the _hashands drift rows.
		// These have the correct identity (clawed/fanged/horned) but also have
		// "hands", so all beast natural-weapon moves must be blocked.
		7310: {SpeciesId: 7310, Name: "hands-clawed-test", BodyParts: []string{"arms", "hands", "legs", "mouth"}, NaturalAttack: items.Claws},
		7311: {SpeciesId: 7311, Name: "hands-fanged-test", BodyParts: []string{"arms", "hands", "legs", "mouth"}, NaturalAttack: items.Bite},
		7312: {SpeciesId: 7312, Name: "hands-horned-test", BodyParts: []string{"arms", "hands", "legs", "mouth", "horns"}, NaturalAttack: items.Gore},
	})
	defer speciesCleanup()

	cases := []driftCase{
		// ─── taunt ────────────────────────────────────────────────
		{"taunt_ready", "taunt",
			func(m *mobs.Mob) { setDriftAggroMobTarget(m, 228) },
			true, ""},
		{"taunt_crafting", "taunt",
			func(m *mobs.Mob) {
				setCraftingForTest(m)
			},
			false, "Crafting"},
		{"taunt_cooldown", "taunt",
			func(m *mobs.Mob) {
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
			},
			false, "OnCooldown"},
		{"taunt_no_aggro", "taunt",
			func(m *mobs.Mob) { m.Character.EndAggro() },
			false, "NoTarget"},

		// ─── rally ────────────────────────────────────────────────
		{"rally_ready", "rally",
			nil,
			true, ""},
		{"rally_crafting", "rally",
			func(m *mobs.Mob) { setCraftingForTest(m) },
			false, "Crafting"},
		{"rally_cooldown", "rally",
			func(m *mobs.Mob) { m.Character.Cooldowns = characters.Cooldowns{"special-move": 3} },
			false, "OnCooldown"},
		{"rally_already_active", "rally",
			func(m *mobs.Mob) { m.Character.AddBuff(80, false) },
			false, "AlreadyActive"},

		// ─── warcry ───────────────────────────────────────────────
		{"warcry_ready", "warcry", nil, true, ""},
		{"warcry_crafting", "warcry",
			func(m *mobs.Mob) { setCraftingForTest(m) },
			false, "Crafting"},
		{"warcry_cooldown", "warcry",
			func(m *mobs.Mob) { m.Character.Cooldowns = characters.Cooldowns{"special-move": 3} },
			false, "OnCooldown"},
		{"warcry_already_active", "warcry",
			func(m *mobs.Mob) { m.Character.AddBuff(79, false) },
			false, "AlreadyActive"},

		// ─── trip ─────────────────────────────────────────────────
		{"trip_ready", "trip",
			func(m *mobs.Mob) {
				// SpeciesId 7300 has legs (seeded above) so the anatomy gate passes.
				m.Character.SpeciesId = 7300
				// Target must be standing (not prone). newTestMob
				// sets default aggro to user 1, but for trip we need
				// a real target mob that's not prone.
				targetMob := &mobs.Mob{InstanceId: 200}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			true, ""},
		{"trip_target_gone", "trip",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7300
				m.Character.SetAggro(0, 999991, characters.DefaultAttack)
			},
			false, "NoTarget"},
		{"trip_target_on_floor", "trip",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7300
				setDriftAggroMobTarget(m, 229)
				target := mobs.GetInstance(229)
				setCombatPositionParallel(&target.Character, position.Prone)
			},
			false, "TargetOnFloor"},
		{"trip_crafting", "trip",
			func(m *mobs.Mob) {
				setCraftingForTest(m)
				targetMob := &mobs.Mob{InstanceId: 201}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "Crafting"},
		{"trip_cooldown", "trip",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7300
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
				setDriftAggroMobTarget(m, 202)
			},
			false, "OnCooldown"},
		{"trip_no_aggro", "trip",
			func(m *mobs.Mob) { m.Character.EndAggro() },
			false, "NoTarget"},

		// ─── bash ─────────────────────────────────────────────────
		// NOTE: bash_cooldown is intentionally omitted. CommandIsReady
		// rejects on the universal cooldown gate first, but ExecuteBash
		// rejects on NoShield first (default test mobs have no shield
		// and no naturalbash). Both agree on the readiness bool, but
		// the reason flag would differ. This test is a readiness-bool
		// agreement test, not a reason-flag agreement test. If
		// ExecuteBash's gate ordering ever changes to put cooldown
		// before NoShield, add the bash_cooldown row back.
		{"bash_ready", "bash",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7308
				setDriftAggroMobTarget(m, 230)
			},
			true, ""},
		{"bash_crafting", "bash",
			func(m *mobs.Mob) {
				setCraftingForTest(m)
			},
			false, "Crafting"},
		{"bash_no_shield", "bash",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 1 // human, no naturalbash, no shield
			},
			false, "NoShield"},
		// SpeciesId 0 → species.GetSpecies(0) returns nil in test context →
		// HasBodyPart("arms") returns false, naturalBash=false. No shield
		// equipped either. Both gates fire; the shield gate fires first so
		// ExecuteBash returns NoShield=true (reused for the anatomy gate too).
		// CommandIsReady likewise returns false. Boolean agreement is the goal.
		{"bash_no_arms", "bash",
			func(m *mobs.Mob) {
				// SpeciesId stays 0 (nil species, no arms, not natural).
				// Default test mob already has no shield and aggro set.
			},
			false, "NoShield"},

		// ─── grapple ──────────────────────────────────────────────
		{"grapple_ready", "grapple",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7307
				setDriftAggroMobTarget(m, 231)
			},
			true, ""},
		{"grapple_crafting", "grapple",
			func(m *mobs.Mob) {
				setCraftingForTest(m)
				targetMob := &mobs.Mob{InstanceId: 204}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "Crafting"},
		{"grapple_cooldown", "grapple",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7307
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
				setDriftAggroMobTarget(m, 205)
			},
			false, "OnCooldown"},
		{"grapple_no_aggro", "grapple",
			func(m *mobs.Mob) { m.Character.EndAggro() },
			false, "NoTarget"},
		{"grapple_target_grappling", "grapple",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7307
				setDriftAggroMobTarget(m, 232)
				target := mobs.GetInstance(232)
				setCombatPositionParallel(&target.Character, position.Clinch)
			},
			false, "TargetGrappling"},
		// SpeciesId 0 → species.GetSpecies(0) returns nil (no species loaded in
		// unit-test context) → HasBodyPart("arms") returns false. A valid
		// non-grappling target is registered so target resolution succeeds and
		// the arms gate is the sole blocking condition in CommandIsReady.
		// ExecuteGrapple checks HasBodyPart before target resolution, so it
		// also fires the arms gate and returns GrappleImmune:true.
		{"grapple_no_arms", "grapple",
			func(m *mobs.Mob) {
				targetMob := &mobs.Mob{InstanceId: 206}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "GrappleImmune"},

		// ─── trip anatomy gate ────────────────────────────────────
		// SpeciesId 0 → species.GetSpecies(0) returns nil (no species loaded in
		// unit-test context) → HasBodyPart("legs") returns false. A valid
		// non-prone target is registered so target resolution succeeds and the
		// legs gate is the sole blocking condition in CommandIsReady.
		// ExecuteTrip also reaches the legs gate (after target resolution) and
		// returns NoTarget:true (reused flag for the unreachable anatomy branch).
		{"trip_no_legs", "trip",
			func(m *mobs.Mob) {
				targetMob := &mobs.Mob{InstanceId: 207}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NoTarget"},

		// ─── kick ─────────────────────────────────────────────────
		{"kick_ready", "kick",
			func(m *mobs.Mob) {
				// SpeciesId 7300 has legs (seeded above) so the anatomy gate passes.
				m.Character.SpeciesId = 7300
				setDriftAggroMobTarget(m, 233)
			},
			true, ""},
		{"kick_crafting", "kick",
			func(m *mobs.Mob) {
				setCraftingForTest(m)
			},
			false, "Crafting"},
		{"kick_cooldown", "kick",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7300
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
				setDriftAggroMobTarget(m, 208)
			},
			false, "OnCooldown"},
		{"kick_no_aggro", "kick",
			func(m *mobs.Mob) { m.Character.EndAggro() },
			false, "NoTarget"},

		// ─── kick anatomy gate ────────────────────────────────────
		// SpeciesId 0 → nil species → no legs. Default mob has aggro set;
		// CommandIsReady("kick") checks Aggro then HasBodyPart("legs") with no
		// target resolution, so the legs gate is the blocking condition.
		// ExecuteKick resolves the target before admission. The missing user
		// returns NoTarget without consuming cost or cooldown.
		{"kick_no_legs", "kick",
			func(m *mobs.Mob) {
				// SpeciesId stays 0 (nil species, no legs). Default aggro to user 1.
			},
			false, "NoTarget"},

		// ─── rake ─────────────────────────────────────────────────
		// rake_ready: clawed species + default aggro (user 1, not found) →
		// CommandIsReady returns true (species gate passes, aggro non-nil).
		// Execute is skipped for ready cases.
		{"rake_ready", "rake",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7304 // clawed-test seeded above
				setDriftAggroMobTarget(m, 234)
			},
			true, ""},
		{"rake_crafting", "rake", func(m *mobs.Mob) { setCraftingForTest(m) }, false, "Crafting"},
		{"rake_cooldown", "rake",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7304
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
				setDriftAggroMobTarget(m, 222)
			},
			false, "OnCooldown"},
		{"rake_no_aggro", "rake",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7304
				m.Character.EndAggro()
			},
			false, "NoTarget"},
		// rake_notclawed: SpeciesId 0 → nil species → not clawed. Default mob
		// has aggro to user 1. CommandIsReady returns false. ExecuteRake burns
		// the cooldown, resolves the target (user 1 not found → NoTarget first),
		// but with a registered target mob we can reach the NotClawed gate.
		// Use a registered mob target so target resolution succeeds and NotClawed fires.
		{"rake_notclawed", "rake",
			func(m *mobs.Mob) {
				// SpeciesId 0 → nil species → not clawed.
				targetMob := &mobs.Mob{InstanceId: 209}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotClawed"},

		// ─── maul ─────────────────────────────────────────────────────────────
		// maul_ready: fanged species + default aggro (user 1, not found) →
		// CommandIsReady returns true (species gate passes, aggro non-nil).
		// Execute is skipped for ready cases.
		{"maul_ready", "maul",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7305 // fanged-test seeded above
				setDriftAggroMobTarget(m, 235)
			},
			true, ""},
		{"maul_crafting", "maul", func(m *mobs.Mob) { setCraftingForTest(m) }, false, "Crafting"},
		{"maul_cooldown", "maul",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7305
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
				setDriftAggroMobTarget(m, 223)
			},
			false, "OnCooldown"},
		{"maul_no_aggro", "maul",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7305
				m.Character.EndAggro()
			},
			false, "NoTarget"},
		// maul_notfanged: SpeciesId 0 → nil species → not fanged. Default mob
		// has aggro to user 1. CommandIsReady returns false. ExecuteMaul burns
		// the cooldown, resolves the target (user 1 not found → NoTarget first),
		// but with a registered target mob we can reach the NotFanged gate.
		// Use a registered mob target so target resolution succeeds and NotFanged fires.
		{"maul_notfanged", "maul",
			func(m *mobs.Mob) {
				// SpeciesId 0 → nil species → not fanged.
				targetMob := &mobs.Mob{InstanceId: 210}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotFanged"},

		// ─── pounce ───────────────────────────────────────────────────────────
		// pounce_ready: clawed legged species + default aggro (user 1, not found)
		// → CommandIsReady returns true (predator gate passes, aggro non-nil).
		// Execute is skipped for ready cases.
		{"pounce_ready", "pounce",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7304 // clawed-test seeded above (has legs + claws)
				setDriftAggroMobTarget(m, 236)
			},
			true, ""},
		{"pounce_crafting", "pounce", func(m *mobs.Mob) { setCraftingForTest(m) }, false, "Crafting"},
		{"pounce_cooldown", "pounce",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7304
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
				setDriftAggroMobTarget(m, 224)
			},
			false, "OnCooldown"},
		{"pounce_no_aggro", "pounce",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7304
				m.Character.EndAggro()
			},
			false, "NoTarget"},
		// pounce_notpredator: SpeciesId 0 → nil species → not a quadruped predator.
		// Default mob has aggro to user 1. CommandIsReady returns false.
		// ExecutePounce resolves the target before admission; a registered target
		// lets the NotPredator validity gate be observed directly.
		{"pounce_notpredator", "pounce",
			func(m *mobs.Mob) {
				// SpeciesId 0 → nil species → not a quadruped predator.
				targetMob := &mobs.Mob{InstanceId: 211}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotPredator"},

		// pounce_grappling: SpeciesId 7304 (predator) but the actor is in
		// Clinch — CommandIsReady gates on !IsGrappling(), ExecutePounce gates
		// after target resolution. A registered target ensures ResolveAggroTarget
		// returns Found=true so the Grappling gate is what fires, not NoTarget.
		{"pounce_grappling", "pounce",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7304 // clawed-test: legs + claws → predator
				// Put the actor into a Clinch grapple so IsGrappling() returns true.
				setCombatPositionParallel(&m.Character, position.Clinch)
				// Register a target so ResolveAggroTarget returns Found=true and
				// the Grappling gate (not NoTarget) is the blocking condition.
				targetMob := &mobs.Mob{InstanceId: 215}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "Grappling"},

		// ─── gore ────────────────────────────────────────────────────────────
		// gore_ready: horned species + default aggro (user 1, not found) →
		// CommandIsReady returns true (horned gate passes, aggro non-nil).
		// Execute is skipped for ready cases.
		{"gore_ready", "gore",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7306 // horned-test seeded above
				setDriftAggroMobTarget(m, 237)
			},
			true, ""},
		{"gore_crafting", "gore", func(m *mobs.Mob) { setCraftingForTest(m) }, false, "Crafting"},
		{"gore_cooldown", "gore",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7306
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
				setDriftAggroMobTarget(m, 225)
			},
			false, "OnCooldown"},
		{"gore_no_aggro", "gore",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7306
				m.Character.EndAggro()
			},
			false, "NoTarget"},
		// gore_nothorned: SpeciesId 0 → nil species → not horned.
		// Default mob has aggro to user 1. CommandIsReady returns false.
		// ExecuteGore resolves the target before admission; a registered target
		// lets the NotHorned validity gate be observed directly.
		{"gore_nothorned", "gore",
			func(m *mobs.Mob) {
				// SpeciesId 0 → nil species → not horned.
				targetMob := &mobs.Mob{InstanceId: 212}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotHorned"},

		// ─── drain ───────────────────────────────────────────────────────────
		// drain_ready: LifeDrain species + default aggro (user 1, not found) →
		// CommandIsReady returns true (lifedrain gate passes, aggro non-nil).
		// Execute is skipped for ready cases.
		{"drain_ready", "drain",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7307 // lifedrain-test seeded above
				setDriftAggroMobTarget(m, 238)
			},
			true, ""},
		{"drain_crafting", "drain", func(m *mobs.Mob) { setCraftingForTest(m) }, false, "Crafting"},
		{"drain_cooldown", "drain",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7307
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
				setDriftAggroMobTarget(m, 226)
			},
			false, "OnCooldown"},
		{"drain_no_aggro", "drain",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7307
				m.Character.EndAggro()
			},
			false, "NoTarget"},
		// drain_notlifedrainer: SpeciesId 0 → nil species → no LifeDrain.
		// Default mob has aggro to user 1. CommandIsReady returns false.
		// ExecuteDrain resolves the target before admission; a registered target
		// lets the NotLifeDrainer validity gate be observed directly.
		{"drain_notlifedrainer", "drain",
			func(m *mobs.Mob) {
				// SpeciesId 0 → nil species → no LifeDrain flag.
				targetMob := &mobs.Mob{InstanceId: 213}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotLifeDrainer"},

		// ─── throttle ─────────────────────────────────────────────────────────
		// throttle_ready: fanged species + default aggro (user 1, not found) →
		// CommandIsReady returns true (species gate passes, aggro non-nil).
		// Execute is skipped for ready cases.
		{"throttle_ready", "throttle",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7305 // fanged-test seeded above
				setDriftAggroMobTarget(m, 239)
			},
			true, ""},
		{"throttle_crafting", "throttle", func(m *mobs.Mob) { setCraftingForTest(m) }, false, "Crafting"},
		{"throttle_cooldown", "throttle",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7305
				m.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
				setDriftAggroMobTarget(m, 227)
			},
			false, "OnCooldown"},
		{"throttle_no_aggro", "throttle",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7305
				m.Character.EndAggro()
			},
			false, "NoTarget"},
		// throttle_notfanged: SpeciesId 0 → nil species → not fanged. Default
		// mob has aggro to user 1. A registered target lets ExecuteThrottle reach
		// the NotFanged validity gate before admission.
		{"throttle_notfanged", "throttle",
			func(m *mobs.Mob) {
				// SpeciesId 0 → nil species → not fanged.
				targetMob := &mobs.Mob{InstanceId: 214}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotFanged"},

		// ─── hamstring ────────────────────────────────────────────────────────
		{"hamstring_ready", "hamstring",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7305
				setDriftAggroMobTarget(m, 240)
			},
			true, ""},
		{"hamstring_crafting", "hamstring", func(m *mobs.Mob) { setCraftingForTest(m) }, false, "Crafting"},
		{"hamstring_no_legs", "hamstring",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7309
				setDriftAggroMobTarget(m, 241)
			},
			false, "NoLegs"},

		// ─── _hashands rows ───────────────────────────────────────────────────
		// Each row seeds a species with the correct beast identity (clawed/fanged/
		// horned) but also with "hands", so CommandIsReady must return false and
		// the Execute* must return the move's Not<Identity> flag. This exercises
		// the true-beast gate added in P4-T1.

		{"rake_hashands", "rake",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7310 // hands+clawed-test
				targetMob := &mobs.Mob{InstanceId: 216}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotClawed"},

		{"maul_hashands", "maul",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7311 // hands+fanged-test
				targetMob := &mobs.Mob{InstanceId: 217}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotFanged"},

		{"gore_hashands", "gore",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7312 // hands+horned-test
				targetMob := &mobs.Mob{InstanceId: 218}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotHorned"},

		{"throttle_hashands", "throttle",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7311 // hands+fanged-test
				targetMob := &mobs.Mob{InstanceId: 219}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotFanged"},

		{"hamstring_hashands", "hamstring",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7311 // hands+fanged+legs-test
				targetMob := &mobs.Mob{InstanceId: 220}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotBeast"},

		{"pounce_hashands", "pounce",
			func(m *mobs.Mob) {
				m.Character.SpeciesId = 7310 // hands+clawed+legs-test
				targetMob := &mobs.Mob{InstanceId: 221}
				targetMob.Character.Name = "Target"
				setCombatPositionParallel(&targetMob.Character, position.Standing)
				mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
				m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
			},
			false, "NotPredator"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up test mobs after each case to avoid pollution.
			// The mutate function may set up target mobs via
			// mobs.SetInstanceForTest; we clean them all up here.
			defer func() {
				for id := 200; id <= 241; id++ {
					mobs.SetInstanceForTest(id, nil)
				}
			}()

			mob := newTestMob(t, tc.mutate)
			actor := &MobActor{Mob: mob, Room: nil}

			gotReady := CommandIsReady(actor, tc.cmd)
			assert.Equal(t, tc.wantReady, gotReady,
				"CommandIsReady(%s) for case %q", tc.cmd, tc.name)

			if tc.wantReady {
				assert.True(t, runExecuteAndReadExecuted(tc.cmd, actor),
					"Execute%s for ready case %q must execute against its real funded target", tc.cmd, tc.name)
				return
			}

			staminaBefore := mob.Character.Stamina
			cooldownsBefore := maps.Clone(mob.Character.Cooldowns)
			roundsBefore := 0
			if mob.Character.Aggro != nil {
				roundsBefore = mob.Character.Aggro.RoundsWaiting
			}
			gotFlag := runExecuteAndReadFlag(tc.cmd, actor, tc.wantReason)
			assert.True(t, gotFlag,
				"Execute%s for case %q did not return %s=true", tc.cmd, tc.name, tc.wantReason)
			physicalSpecial := map[string]bool{
				"bash": true, "trip": true, "kick": true, "grapple": true, "hamstring": true,
				"rake": true, "maul": true, "pounce": true, "gore": true, "drain": true, "throttle": true,
			}
			if physicalSpecial[tc.cmd] {
				assert.Equal(t, staminaBefore, mob.Character.Stamina,
					"readiness rejection %q must precede cost admission", tc.name)
				assert.Equal(t, cooldownsBefore, mob.Character.Cooldowns,
					"readiness rejection %q must not consume or rewrite cooldown", tc.name)
				if mob.Character.Aggro != nil {
					assert.Equal(t, roundsBefore, mob.Character.Aggro.RoundsWaiting,
						"readiness rejection %q must not consume a round", tc.name)
				}
			}
		})
	}
}

// runExecuteAndReadFlag dispatches to the Execute* matching cmd and
// returns whether the named result field is true.
func runExecuteAndReadFlag(cmd string, actor Actor, flag string) bool {
	switch cmd {
	case "taunt":
		r := ExecuteTaunt(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		}
	case "rally":
		r := ExecuteRally(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "AlreadyActive":
			return r.AlreadyActive
		}
	case "warcry":
		r := ExecuteWarcry(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "AlreadyActive":
			return r.AlreadyActive
		}
	case "trip":
		r := ExecuteTrip(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		case "TargetOnFloor":
			return r.TargetOnFloor
		}
	case "bash":
		r := ExecuteBash(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoShield":
			return r.NoShield
		}
	case "grapple":
		r := ExecuteGrapple(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		case "GrappleImmune":
			return r.GrappleImmune
		case "TargetGrappling":
			return r.TargetGrappling
		}
	case "kick":
		r := ExecuteKick(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		}
	case "rake":
		r := ExecuteRake(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		case "NotClawed":
			return r.NotClawed
		}
	case "maul":
		r := ExecuteMaul(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		case "NotFanged":
			return r.NotFanged
		}
	case "pounce":
		r := ExecutePounce(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		case "Grappling":
			return r.Grappling
		case "NotPredator":
			return r.NotPredator
		}
	case "gore":
		r := ExecuteGore(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		case "NotHorned":
			return r.NotHorned
		}
	case "drain":
		r := ExecuteDrain(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		case "NotLifeDrainer":
			return r.NotLifeDrainer
		}
	case "throttle":
		r := ExecuteThrottle(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		case "NotFanged":
			return r.NotFanged
		}
	case "hamstring":
		r := ExecuteHamstring(actor)
		switch flag {
		case "Crafting":
			return r.Crafting
		case "OnCooldown":
			return r.OnCooldown
		case "NoTarget":
			return r.NoTarget
		case "NotBeast":
			return r.NotBeast
		case "NoLegs":
			return r.NoLegs
		}
	}
	return false
}

// setCraftingForTest puts a mob into crafting state.
func setCraftingForTest(m *mobs.Mob) {
	m.Character.Activity = activity.NewMachine()
	_ = m.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: "test-recipe", RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
}
