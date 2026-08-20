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
	file              string
	function          string
	resultType        string
	action            string
	earlyReturns      []specialMoveEarlyReturn
	postCommitCallees []string
	roundAssignment   bool
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

func compositeReturnHasTrueField(stmt ast.Stmt, resultType, field string) bool {
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	lit, ok := ret.Results[0].(*ast.CompositeLit)
	if !ok {
		return false
	}
	typ, ok := lit.Type.(*ast.Ident)
	if !ok || typ.Name != resultType {
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

func compositeReturnHasExactFields(t *testing.T, fset *token.FileSet, stmt ast.Stmt, resultType string, fields map[string]string) bool {
	t.Helper()
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	lit, ok := ret.Results[0].(*ast.CompositeLit)
	if !ok || len(lit.Elts) != len(fields) {
		return false
	}
	typ, ok := lit.Type.(*ast.Ident)
	if !ok || typ.Name != resultType {
		return false
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return false
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || fields[key.Name] != formattedASTNode(t, fset, kv.Value) {
			return false
		}
	}
	return true
}

func exactGuardedReturn(t *testing.T, fset *token.FileSet, body *ast.BlockStmt, condition, resultType string, fields map[string]string) *ast.IfStmt {
	t.Helper()
	matches := []*ast.IfStmt{}
	for _, stmt := range body.List {
		ifStmt, ok := stmt.(*ast.IfStmt)
		if !ok || formattedASTNode(t, fset, ifStmt.Cond) != condition || len(ifStmt.Body.List) != 1 {
			continue
		}
		if compositeReturnHasExactFields(t, fset, ifStmt.Body.List[0], resultType, fields) {
			matches = append(matches, ifStmt)
		}
	}
	require.Len(t, matches, 1, "%s must guard exact typed return %s%v", condition, resultType, fields)
	return matches[0]
}

func exactCallPositions(t *testing.T, fset *token.FileSet, body *ast.BlockStmt, want string, calleeOnly bool) []token.Pos {
	t.Helper()
	positions := []token.Pos{}
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		var candidate ast.Node = call
		if calleeOnly {
			candidate = call.Fun
		}
		if formattedASTNode(t, fset, candidate) == want {
			positions = append(positions, call.Pos())
		}
		return true
	})
	return positions
}

// TestSpecialMoveAdmissionOrdering catches a validity branch, read-only
// cooldown probe, cost admission, or consuming cooldown being moved across the
// required boundary. It walks each function's Go AST, matches exact guard
// expressions and their immediate typed early returns, checks every occurrence
// of the ordering calls, and proves resolution/effect/round/progression nodes
// remain after the consuming cooldown. Selector-name presence cannot satisfy it.
func TestSpecialMoveAdmissionOrdering(t *testing.T) {
	guards := []specialMoveOrderGuard{
		{"combat_bash.go", "ExecuteBash", "BashResult", "costs.ActionBash", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"!char.HasShield() && !naturalBash", "NoShield"}, {"!char.HasBodyPart(\"arms\") && !naturalBash", "NoShield"},
		}, []string{"combat.ExecuteSkillMove", "RecordAndWait", "actor.OnSkillUse"}, false},
		{"combat_trip.go", "ExecuteTrip", "TripResult", "costs.ActionTrip", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!char.HasBodyPart(\"legs\")", "NoTarget"},
			{"target.Char.IsOnFloor()", "TargetOnFloor"},
		}, []string{"combat.ExecuteSkillMove", "RecordAndWait", "actor.OnSkillUse"}, false},
		{"combat_kick.go", "ExecuteKick", "KickResult", "costs.ActionKick", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!char.HasBodyPart(\"legs\")", "NoTarget"},
		}, []string{"combat.ExecuteSkillMove", "RecordAndWait", "actor.OnSkillUse"}, false},
		{"combat_grapple.go", "ExecuteGrapple", "GrappleResult", "costs.ActionGrapple", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!char.HasBodyPart(\"arms\")", "GrappleImmune"}, {"!target.Found", "NoTarget"},
			{"target.Char.IsGrappling()", "TargetGrappling"},
		}, []string{"combat.ExecuteGrappleMove", "RecordAndWait", "actor.OnSkillUse"}, false},
		{"combat_hamstring.go", "ExecuteHamstring", "HamstringResult", "costs.ActionHamstring", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!char.HasBodyPart(\"legs\")", "NoLegs"},
			{"sp == nil || (sp.NaturalAttack != items.Bite && sp.NaturalAttack != items.Claws) || char.HasBodyPart(\"hands\")", "NotBeast"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddCondition", "combat.RecordSpecialMove", "actor.OnSkillUse"}, true},
		{"combat_rake.go", "ExecuteRake", "RakeResult", "costs.ActionRake", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsClawed(char)", "NotClawed"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddCondition", "combat.RecordSpecialMove", "actor.OnSkillUse"}, true},
		{"combat_maul.go", "ExecuteMaul", "MaulResult", "costs.ActionMaul", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsFanged(char)", "NotFanged"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddCondition", "combat.RecordSpecialMove", "actor.OnSkillUse"}, true},
		{"combat_pounce.go", "ExecutePounce", "PounceResult", "costs.ActionPounce", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"char.IsGrappling()", "Grappling"},
			{"!combat.SpeciesIsQuadrupedPredator(char)", "NotPredator"},
		}, []string{"combat.ExecuteSkillMove", "combat.RecordSpecialMove", "actor.OnSkillUse"}, true},
		{"combat_gore.go", "ExecuteGore", "GoreResult", "costs.ActionGore", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsHorned(char)", "NotHorned"},
		}, []string{"combat.ExecuteSkillMove", "combat.RecordSpecialMove", "actor.OnSkillUse"}, true},
		{"combat_drain.go", "ExecuteDrain", "DrainResult", "costs.ActionDrain", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!combat.SpeciesHasLifeDrain(char)", "NotLifeDrainer"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddCondition", "char.Heal", "combat.RecordSpecialMove", "actor.OnSkillUse"}, true},
		{"combat_throttle.go", "ExecuteThrottle", "ThrottleResult", "costs.ActionThrottle", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsFanged(char)", "NotFanged"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddCondition", "target.Char.AddBuff", "InterruptTargetCast", "combat.RecordSpecialMove", "actor.OnSkillUse"}, true},
	}

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	actionsDir := filepath.Dir(thisFile)

	for _, guard := range guards {
		t.Run(guard.function, func(t *testing.T) {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, filepath.Join(actionsDir, guard.file), nil, 0)
			require.NoError(t, err)

			var body *ast.BlockStmt
			for _, decl := range parsed.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Name.Name == guard.function {
					body = fn.Body
					break
				}
			}
			require.NotNil(t, body, "%s must exist", guard.function)
			charAssignments := 0
			exactCharAssignment := 0
			cfgAssignments := 0
			exactCfgAssignment := 0
			for _, stmt := range body.List {
				assign, ok := stmt.(*ast.AssignStmt)
				if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
					continue
				}
				switch formattedASTNode(t, fset, assign.Lhs[0]) {
				case "char":
					charAssignments++
					if formattedASTNode(t, fset, assign.Rhs[0]) == "actor.GetCharacter()" {
						exactCharAssignment++
					}
				case "cfg":
					cfgAssignments++
					if formattedASTNode(t, fset, assign.Rhs[0]) == "configs.GetBalanceConfig()" {
						exactCfgAssignment++
					}
				}
			}
			require.Equal(t, 1, charAssignments, "%s must bind char exactly once", guard.function)
			require.Equal(t, 1, exactCharAssignment, "%s char receiver must be the supplied actor's character", guard.function)
			require.Equal(t, 1, cfgAssignments, "%s must bind cfg exactly once", guard.function)
			require.Equal(t, 1, exactCfgAssignment, "%s cooldown/base config must be the live balance config", guard.function)
			readyCalls := exactCallPositions(t, fset, body, `char.CooldownReady("special-move")`, false)
			admitCalls := exactCallPositions(t, fset, body,
				"admitFullCost(actor, "+guard.action+", characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))", false)
			consumeCalls := exactCallPositions(t, fset, body,
				`char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown))`, false)
			require.Len(t, readyCalls, 1, "%s must use the actor character and exact special-move tag for its read-only probe", guard.function)
			require.Len(t, admitCalls, 1, "%s must admit the exact action, stamina pool, and configured base", guard.function)
			require.Len(t, consumeCalls, 1, "%s must use the actor character, exact tag, and configured duration for consuming cooldown", guard.function)
			ready := readyCalls[0]
			admit := admitCalls[0]
			consume := consumeCalls[0]
			require.Less(t, int(ready), int(admit), "CooldownReady must precede admission")
			require.Less(t, int(admit), int(consume), "admission must precede consuming TryCooldown")

			resolveCalls := exactCallPositions(t, fset, body, "resolveActionTarget(actor, char)", false)
			require.Len(t, resolveCalls, 1, "%s must resolve through the staged-target-aware seam with actor and character", guard.function)
			require.Less(t, int(resolveCalls[0]), int(ready), "target resolution must precede cooldown/admission")
			if guard.function != "ExecuteHamstring" {
				commitCalls := exactCallPositions(t, fset, body, "commitMeleeEngagement(actor)", false)
				require.Len(t, commitCalls, 1, "%s must commit the staged actor once", guard.function)
				require.Greater(t, int(commitCalls[0]), int(consume),
					"%s must not commit engagement before successful admission/cooldown", guard.function)
			}

			for _, expected := range guard.earlyReturns {
				found := false
				for _, stmt := range body.List {
					ifStmt, ok := stmt.(*ast.IfStmt)
					if !ok || formattedASTNode(t, fset, ifStmt.Cond) != expected.condition || len(ifStmt.Body.List) != 1 {
						continue
					}
					if compositeReturnHasTrueField(ifStmt.Body.List[0], guard.resultType, expected.field) {
						found = true
						require.Less(t, int(ifStmt.Pos()), int(ready), "%s guard %s must precede CooldownReady", guard.function, expected.condition)
					}
				}
				require.True(t, found, "%s must have exact early return `if %s { return ...{%s: true} }`", guard.function, expected.condition, expected.field)
			}

			for _, callee := range guard.postCommitCallees {
				positions := exactCallPositions(t, fset, body, callee, true)
				require.NotEmpty(t, positions, "%s must retain exact call identity %s", guard.function, callee)
				for _, pos := range positions {
					require.Greater(t, int(pos), int(consume), "%s call %s must follow consuming TryCooldown", guard.function, callee)
					if guard.function != "ExecuteHamstring" {
						commitCalls := exactCallPositions(t, fset, body, "commitMeleeEngagement(actor)", false)
						require.Greater(t, int(pos), int(commitCalls[0]),
							"%s call %s must follow engagement commit", guard.function, callee)
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
					if assign.Tok == token.ASSIGN && len(assign.Lhs) == 1 && len(assign.Rhs) == 1 &&
						formattedASTNode(t, fset, assign.Lhs[0]) == "char.Aggro.RoundsWaiting" &&
						formattedASTNode(t, fset, assign.Rhs[0]) == "1" {
						roundPositions = append(roundPositions, assign.Pos())
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

// TestTauntRallyWarcryAdmissionOrdering guards the social-action mutation
// boundary. It requires exact read-only validity/cooldown probes before the
// Conviction admission and exact reveal/cooldown/effect/progression calls only
// after admission. Taunt additionally resolves through the staged target seam,
// revalidates after payment, and commits engagement only after cooldown
// consumption.
func TestTauntRallyWarcryAdmissionOrdering(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	actionsDir := filepath.Dir(thisFile)

	type rhetoricOrderGuard struct {
		file       string
		function   string
		resultType string
		action     string
		guards     []specialMoveEarlyReturn
		effect     string
	}
	guards := []rhetoricOrderGuard{
		{
			file: "combat_taunt.go", function: "ExecuteTaunt", resultType: "TauntResult", action: "costs.ActionTaunt",
			guards: []specialMoveEarlyReturn{{"char.IsActing()", "Crafting"}, {"!tauntTargetIsCurrent(target, target, originalRoomID, char)", "NoTarget"}},
			effect: "combat.ResolveChannelAttack",
		},
		{
			file: "combat_rally.go", function: "ExecuteRally", resultType: "RallyResult", action: "costs.ActionRally",
			guards: []specialMoveEarlyReturn{{"char.IsActing()", "Crafting"}, {"char.HasBuff(80)", "AlreadyActive"}},
			effect: "ApplyRallyEffect",
		},
		{
			file: "combat_warcry.go", function: "ExecuteWarcry", resultType: "WarcryResult", action: "costs.ActionWarcry",
			guards: []specialMoveEarlyReturn{{"char.IsActing()", "Crafting"}, {"char.HasBuff(79)", "AlreadyActive"}},
			effect: "ApplyWarcryEffect",
		},
	}

	for _, guard := range guards {
		t.Run(guard.function, func(t *testing.T) {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, filepath.Join(actionsDir, guard.file), nil, 0)
			require.NoError(t, err)
			var fn *ast.FuncDecl
			for _, decl := range parsed.Decls {
				candidate, ok := decl.(*ast.FuncDecl)
				if ok && candidate.Name.Name == guard.function {
					fn = candidate
					break
				}
			}
			require.NotNil(t, fn)

			ready := exactCallPositions(t, fset, fn.Body, `char.CooldownReady("special-move")`, false)
			admit := exactCallPositions(t, fset, fn.Body, "admitFullCost", true)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || formattedASTNode(t, fset, call.Fun) != "admitFullCost" {
					return true
				}
				require.Len(t, call.Args, 4)
				require.Equal(t, "actor", formattedASTNode(t, fset, call.Args[0]))
				require.Equal(t, guard.action, formattedASTNode(t, fset, call.Args[1]))
				require.Equal(t, "characters.PoolConviction", formattedASTNode(t, fset, call.Args[2]))
				require.Equal(t, "float64(cfg.RhetoricActionBaseConvictionCost)", formattedASTNode(t, fset, call.Args[3]))
				return false
			})
			consume := exactCallPositions(t, fset, fn.Body,
				`char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown))`, false)
			reveal := exactCallPositions(t, fset, fn.Body, "char.Awareness.TransitionToRevealing", true)
			effects := exactCallPositions(t, fset, fn.Body, guard.effect, true)
			progression := exactCallPositions(t, fset, fn.Body, "actor.OnSkillUse", true)
			if guard.function != "ExecuteTaunt" {
				progression = exactCallPositions(t, fset, fn.Body, "awardRhetoricUse", true)
			}
			require.Len(t, ready, 1)
			require.Len(t, admit, 1)
			require.Len(t, consume, 1)
			require.Len(t, reveal, 1)
			require.NotEmpty(t, effects)
			require.NotEmpty(t, progression)
			require.Less(t, int(ready[0]), int(admit[0]))
			require.Less(t, int(admit[0]), int(consume[0]))
			require.Less(t, int(consume[0]), int(reveal[0]))

			readyBranch := exactGuardedReturn(t, fset, fn.Body,
				`!char.CooldownReady("special-move")`, guard.resultType,
				map[string]string{"OnCooldown": "true"})
			refusalBranch := exactGuardedReturn(t, fset, fn.Body,
				"cost.Status == characters.CostRefused", guard.resultType,
				map[string]string{"Cost": "cost"})
			consumeBranch := exactGuardedReturn(t, fset, fn.Body,
				`!char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown))`, guard.resultType,
				map[string]string{"Cost": "cost", "OnCooldown": "true"})
			require.Less(t, int(readyBranch.Pos()), int(admit[0]))
			require.Less(t, int(admit[0]), int(refusalBranch.Pos()))
			require.Less(t, int(refusalBranch.Pos()), int(consumeBranch.Pos()))
			require.Less(t, int(consumeBranch.Pos()), int(reveal[0]))
			for _, pos := range effects {
				require.Less(t, int(consume[0]), int(pos))
			}
			for _, pos := range progression {
				require.Less(t, int(consume[0]), int(pos))
			}

			for _, expected := range guard.guards {
				found := false
				for _, stmt := range fn.Body.List {
					ifStmt, ok := stmt.(*ast.IfStmt)
					if !ok || formattedASTNode(t, fset, ifStmt.Cond) != expected.condition || len(ifStmt.Body.List) != 1 {
						continue
					}
					if compositeReturnHasTrueField(ifStmt.Body.List[0], guard.resultType, expected.field) {
						found = found || ifStmt.Pos() < ready[0]
					}
				}
				require.True(t, found, "%s must keep exact read-only guard %s", guard.function, expected.condition)
			}

			roundPositions := []token.Pos{}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				assign, ok := node.(*ast.AssignStmt)
				if ok && assign.Tok == token.ASSIGN && len(assign.Lhs) == 1 && len(assign.Rhs) == 1 &&
					formattedASTNode(t, fset, assign.Lhs[0]) == "char.Aggro.RoundsWaiting" &&
					formattedASTNode(t, fset, assign.Rhs[0]) == "1" {
					roundPositions = append(roundPositions, assign.Pos())
				}
				return true
			})
			require.NotEmpty(t, roundPositions)
			for _, pos := range roundPositions {
				require.Less(t, int(consume[0]), int(pos))
			}

			if guard.function == "ExecuteTaunt" {
				resolve := exactCallPositions(t, fset, fn.Body, "resolveActionTarget(actor, char)", false)
				commit := exactCallPositions(t, fset, fn.Body, "commitMeleeEngagement(actor)", false)
				require.Len(t, resolve, 2, "taunt must validate before admission and revalidate after payment")
				require.Len(t, commit, 1)
				require.Less(t, int(resolve[0]), int(ready[0]))
				require.Less(t, int(admit[0]), int(resolve[1]))
				require.Less(t, int(resolve[1]), int(consume[0]))
				staleTargetBranch := exactGuardedReturn(t, fset, fn.Body,
					"!tauntTargetIsCurrent(targetSnapshot, target, originalRoomID, char)", "TauntResult",
					map[string]string{"Cost": "cost", "NoTarget": "true"})
				require.Less(t, int(resolve[1]), int(staleTargetBranch.Pos()))
				require.Less(t, int(staleTargetBranch.Pos()), int(consume[0]))
				require.Less(t, int(consume[0]), int(commit[0]))
				for _, pos := range effects {
					require.Less(t, int(commit[0]), int(pos))
				}
			}
		})
	}
}

// TestTauntRallyWarcryWrapperAdmission proves the exact public/private split:
// players render the shared Conviction refusal and return, while mob wrappers
// compare the same structured result and return silently. Taunt must enter the
// existing staged target seam instead of eager acquisition.
func TestTauntRallyWarcryWrapperAdmission(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	type wrapperGuard struct {
		dir, file, function, execute string
		player, staged               bool
	}
	wrappers := []wrapperGuard{
		{"usercommands", "taunt", "Taunt", "executeTauntAction(actor)", true, true},
		{"usercommands", "rally", "Rally", "actions.ExecuteRally(&actions.UserActor{User: user, Room: room})", true, false},
		{"usercommands", "warcry", "Warcry", "actions.ExecuteWarcry(&actions.UserActor{User: user, Room: room})", true, false},
		{"mobcommands", "taunt", "Taunt", "executeTauntAction(actor)", false, false},
		{"mobcommands", "howl", "Howl", "executeTauntAction(actor)", false, false},
		{"mobcommands", "rally", "Rally", "actions.ExecuteRally(&actions.MobActor{Mob: mob, Room: room})", false, false},
		{"mobcommands", "warcry", "Warcry", "actions.ExecuteWarcry(&actions.MobActor{Mob: mob, Room: room})", false, false},
	}

	for _, guard := range wrappers {
		t.Run(guard.dir+"/"+guard.function, func(t *testing.T) {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, filepath.Join(repoRoot, "internal", guard.dir, guard.file+".go"), nil, 0)
			require.NoError(t, err)
			var fn *ast.FuncDecl
			for _, decl := range parsed.Decls {
				candidate, ok := decl.(*ast.FuncDecl)
				if ok && candidate.Name.Name == guard.function {
					fn = candidate
					break
				}
			}
			require.NotNil(t, fn)
			require.Len(t, exactCallPositions(t, fset, fn.Body, guard.execute, false), 1)

			var refusal *ast.IfStmt
			for _, stmt := range fn.Body.List {
				ifStmt, ok := stmt.(*ast.IfStmt)
				if ok && formattedASTNode(t, fset, ifStmt.Cond) == "result.Cost.Status == characters.CostRefused" {
					refusal = ifStmt
					break
				}
			}
			require.NotNil(t, refusal)
			if guard.player {
				require.Len(t, refusal.Body.List, 2)
				require.True(t, nodeHasCall(refusal.Body.List[0], "SendText"))
				require.True(t, nodeHasCall(refusal.Body.List[0], "CostRefusalText"))
				require.True(t, handledReturn(refusal.Body.List[1]))
			} else {
				require.Len(t, refusal.Body.List, 1)
				require.True(t, handledReturn(refusal.Body.List[0]))
				require.False(t, nodeHasCall(fn.Body, "CostRefusalText"))
			}
			if guard.staged {
				require.Len(t, exactCallPositions(t, fset, fn.Body, "stageSpecialMoveTarget", true), 1)
				require.False(t, nodeHasCall(fn.Body, "AcquireMeleeTarget"))
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
				stageIndex := -1
				for i, stmt := range fn.Body.List {
					assign, ok := stmt.(*ast.AssignStmt)
					if !ok || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
						continue
					}
					actorName, actorOK := assign.Lhs[0].(*ast.Ident)
					handledName, handledOK := assign.Lhs[1].(*ast.Ident)
					call, callOK := assign.Rhs[0].(*ast.CallExpr)
					if actorOK && handledOK && callOK && actorName.Name == "actor" && handledName.Name == "handled" &&
						formattedASTNode(t, fset, call.Fun) == "stageSpecialMoveTarget" {
						stageIndex = i
						break
					}
				}
				require.GreaterOrEqual(t, stageIndex, 0, "%s must assign actor, handled from stageSpecialMoveTarget", function)
				require.Less(t, stageIndex+1, len(fn.Body.List), "%s must branch immediately after staging", function)
				handledBranch, ok := fn.Body.List[stageIndex+1].(*ast.IfStmt)
				require.True(t, ok, "%s must branch on handled immediately after staging", function)
				require.Equal(t, "handled", formattedASTNode(t, fset, handledBranch.Cond))
				require.Len(t, handledBranch.Body.List, 1)
				require.True(t, handledReturn(handledBranch.Body.List[0]))
				require.False(t, nodeHasCall(fn.Body, "AcquireMeleeTarget"), "%s must not eagerly acquire/engage", function)
				require.False(t, nodeHasCall(fn.Body, "StageMeleeTarget"), "%s must not bypass the shared wrapper staging seam", function)

				executeName := "actions.Execute" + function
				executeAssignment := false
				for _, stmt := range fn.Body.List {
					assign, ok := stmt.(*ast.AssignStmt)
					if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 || formattedASTNode(t, fset, assign.Lhs[0]) != resultName {
						continue
					}
					call, ok := assign.Rhs[0].(*ast.CallExpr)
					if ok && formattedASTNode(t, fset, call.Fun) == executeName && len(call.Args) == 1 &&
						formattedASTNode(t, fset, call.Args[0]) == "actor" {
						executeAssignment = true
					}
				}
				require.True(t, executeAssignment, "%s must pass its staged actor directly to %s", function, executeName)
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
				setDriftAggroMobTarget(m, 242)
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
				for id := 200; id <= 242; id++ {
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
			convictionBefore := mob.Character.Conviction
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
			rhetoricAction := map[string]bool{"taunt": true, "rally": true, "warcry": true}
			if rhetoricAction[tc.cmd] {
				assert.Equal(t, convictionBefore, mob.Character.Conviction,
					"readiness rejection %q must precede Conviction admission", tc.name)
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
