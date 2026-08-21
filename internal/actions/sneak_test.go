package actions

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CalcSneakScore Tests
// ---------------------------------------------------------------------------

// TestCalcSneakScore_Baseline verifies that a character with Dexterity 100
// and skullduggery 0 produces a reasonable sneak score in a dark room
// (effectiveLit=false, no emit). This is the baseline with no modifier.
func TestCalcSneakScore_Baseline(t *testing.T) {
	char := newTestChar()
	// newTestChar() creates a character with default stats (100 each)
	char.Stats.Dexterity.ValueAdj = 100

	score := CalcSneakScore(char, false /* effectiveLit */)

	// With Dex 100 and skill 0, multiplier is base (1.0), so:
	// score = 100 + 1.0 * 25 = 125, plus any bonuses
	assert.Greater(t, score, 100.0, "sneak score should be > 100 with Dex 100")
	// Allow for stat bonuses and mutations
	assert.Less(t, score, 150.0, "sneak score should be reasonable with no skill")
}

// TestCalcSneakScore_WithSkill verifies that higher skullduggery skill
// increases the sneak score.
func TestCalcSneakScore_WithSkill(t *testing.T) {
	charLow := newTestChar()
	charLow.Stats.Dexterity.ValueAdj = 100
	// Default skill is 0

	charHigh := newTestChar()
	charHigh.Stats.Dexterity.ValueAdj = 100
	// Set skullduggery to a moderate level (e.g., 20)
	charHigh.Skills[string(skills.Skullduggery)] = 20

	scoreLow := CalcSneakScore(charLow, false)
	scoreHigh := CalcSneakScore(charHigh, false)

	assert.Less(t, scoreLow, scoreHigh,
		"sneak score with skill should be higher than sneak score without skill")
}

// ---------------------------------------------------------------------------
// AW-024 through AW-027: Light-conditional sneak score tests
// (AW-024/025 implemented here; AW-026/027 skipped — require EmitsLight=true
// which needs buff or equipment setup beyond unit-test scope.)
// ---------------------------------------------------------------------------

// AW-024: Baseline — dark sneaker, dark room (effectiveLit=false). No modifier.
func TestCalcSneakScore_AW024_BaselineDarkRoom(t *testing.T) {
	char := newTestChar()
	char.Stats.Dexterity.ValueAdj = 100

	scoreBaseline := CalcSneakScore(char, false /* effectiveLit */)

	// Expected: Dex(100) + SkillMult(0)*25 = 100 + 25 = 125, plus mutation bonus
	// No light modifier applies (dark sneaker, dark room).
	assert.Greater(t, scoreBaseline, 100.0, "baseline dark-room score should exceed Dex")

	// Confirm no penalty relative to lit-room variant: baseline >= lit-room score.
	scoreLitRoom := CalcSneakScore(char, true /* effectiveLit */)
	assert.GreaterOrEqual(t, scoreBaseline, scoreLitRoom,
		"dark room should yield >= sneak score compared to lit room (no penalty)")
}

// AW-025: Lit room, dark sneaker — SneakModNoLightLitRoom (default 0.9).
func TestCalcSneakScore_AW025_LitRoom(t *testing.T) {
	char := newTestChar()
	char.Stats.Dexterity.ValueAdj = 100

	scoreDark := CalcSneakScore(char, false /* effectiveLit */)
	scoreLit := CalcSneakScore(char, true /* effectiveLit */)

	// Lit room applies 0.9x modifier — score should be ~90% of dark baseline.
	assert.Less(t, scoreLit, scoreDark,
		"lit room should reduce sneak score vs dark room (alert observers)")
	assert.InDelta(t, scoreDark*0.9, scoreLit, 0.01,
		"lit-room modifier should be exactly 0.9× of dark-room baseline")
}

// ---------------------------------------------------------------------------
// CalcSearchScore Tests
// ---------------------------------------------------------------------------

// TestCalcSearchScore_Baseline verifies that a character with Perception 100
// and search 0 produces a reasonable search score.
func TestCalcSearchScore_Baseline(t *testing.T) {
	char := newTestChar()
	// newTestChar() creates a character with default stats (100 each)
	char.Stats.Perception.ValueAdj = 100

	score := CalcSearchScore(char)

	// With Perception 100 and skill 0, multiplier is base (1.0), so:
	// score = 100 + 1.0 * 25 = 125, plus any bonuses
	assert.Greater(t, score, 100.0, "search score should be > 100 with Perception 100")
	// Allow for stat bonuses
	assert.Less(t, score, 150.0, "search score should be reasonable with no skill")
}

// ---------------------------------------------------------------------------
// Sneak Tests
// ---------------------------------------------------------------------------

func assertSneakRefusalPreservesCharacter(t *testing.T, char *characters.Character, result SneakResult) {
	t.Helper()
	require.Equal(t, characters.CostRefused, result.Cost.Status)
	assert.Equal(t, characters.PoolStamina, result.Cost.Pool)
	assert.False(t, result.Success)
	assert.False(t, result.RollHappened)
	assert.Equal(t, awareness.Visible, char.Awareness.State())
	assert.False(t, char.IsHidden())
	assert.Nil(t, char.GetMiscData("sneaking"))
	assert.Zero(t, char.GetSkillUseCount(string(skills.Skullduggery)))
}

func TestSneakUserCostRefusalPreservesAwarenessCooldownProgressionAndCarry(t *testing.T) {
	user := users.NewTestUser(79071, "sneaker", "Sneaker", 0)
	user.Character.Stamina = 0
	user.Character.Cooldowns = characters.Cooldowns{"skullduggery-sneak": -2, "other": 7}
	cooldownsBefore := user.Character.GetAllCooldowns()

	result := Sneak(&UserActor{User: user, Room: newTestRoom()})

	assertSneakRefusalPreservesCharacter(t, user.Character, result)
	assert.Equal(t, cooldownsBefore, user.Character.GetAllCooldowns())
	assert.Zero(t, user.Character.Stamina)

	// A refused 2.75 quote must not advance carry. Two later rank-zero sneak
	// admissions therefore charge 2 then 3, rather than 3 then 3.
	user.Character.Stamina = 100
	charged := make([]int, 0, 2)
	for range 2 {
		quote := user.Character.QuoteActionCost(characters.ActionCostRequest{
			Action: costs.ActionSneak, Pool: characters.PoolStamina,
			Base: float64(configs.GetBalanceConfig().SneakBaseStaminaCost), Modifier: 1, Units: 1,
		})
		charged = append(charged, user.Character.CommitCost(quote, characters.CostFullOrRefuse).Charged)
	}
	assert.Equal(t, []int{2, 3}, charged)
}

func TestSneakMobCostRefusalPreservesAwarenessCooldownAndProgression(t *testing.T) {
	char := characters.New()
	char.Name = "Sneaking mob"
	char.Stamina = 0
	char.IsMob = true
	char.Cooldowns = characters.Cooldowns{"skullduggery-sneak": 9, "other": 4}
	cooldownsBefore := char.GetAllCooldowns()
	mob := &mobs.Mob{MobId: 1, InstanceId: 79072, Character: *char}

	result := Sneak(&MobActor{Mob: mob, Room: newTestRoom()})

	assertSneakRefusalPreservesCharacter(t, &mob.Character, result)
	assert.Equal(t, cooldownsBefore, mob.Character.GetAllCooldowns())
	assert.Zero(t, mob.Character.Stamina)
}

func task6FunctionAST(t *testing.T, path, name string) (*token.FileSet, *ast.FuncDecl) {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, nil, 0)
	require.NoError(t, err)
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fset, fn
		}
	}
	require.FailNow(t, "function not found", "%s in %s", name, path)
	return nil, nil
}

func task6OnlyCall(t *testing.T, fset *token.FileSet, fn *ast.FuncDecl, call string, calleeOnly bool) token.Pos {
	t.Helper()
	positions := exactCallPositions(t, fset, fn.Body, call, calleeOnly)
	require.Len(t, positions, 1, "%s must contain exactly one %s call", fn.Name.Name, call)
	return positions[0]
}

// TestThrowSneakCostAdmissionOrdering guards the mutation boundary itself.
// Behavioral tests prove final state; this AST test catches a future edit that
// briefly transitions awareness or consumes an item/cooldown before admission.
func TestThrowSneakCostAdmissionOrdering(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	actionsDir := filepath.Dir(thisFile)

	t.Run("shared sneak", func(t *testing.T) {
		fset, fn := task6FunctionAST(t, filepath.Join(actionsDir, "sneak.go"), "Sneak")
		free := task6OnlyCall(t, fset, fn, "char.IsFree()", false)
		room := task6OnlyCall(t, fset, fn, "actor.GetRoom()", false)
		admit := task6OnlyCall(t, fset, fn, "admitFullCost", true)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || formattedASTNode(t, fset, call.Fun) != "admitFullCost" {
				return true
			}
			require.Len(t, call.Args, 4)
			assert.Equal(t, "actor", formattedASTNode(t, fset, call.Args[0]))
			assert.Equal(t, "costs.ActionSneak", formattedASTNode(t, fset, call.Args[1]))
			assert.Equal(t, "characters.PoolStamina", formattedASTNode(t, fset, call.Args[2]))
			assert.Equal(t, "float64(cfg.SneakBaseStaminaCost)", formattedASTNode(t, fset, call.Args[3]))
			return false
		})
		transition := task6OnlyCall(t, fset, fn, "char.Awareness.TransitionToConcealing", true)
		resolve := exactCallPositions(t, fset, fn.Body, "char.Awareness.ResolveConcealment", true)
		require.NotEmpty(t, resolve)
		require.Less(t, int(free), int(admit))
		require.Less(t, int(room), int(admit))
		require.Less(t, int(admit), int(transition))
		for _, pos := range resolve {
			require.Less(t, int(transition), int(pos))
		}
	})

	t.Run("player wrapper", func(t *testing.T) {
		path := filepath.Join(actionsDir, "..", "usercommands", "skill.skullduggery.sneak.go")
		fset, fn := task6FunctionAST(t, path, "Sneak")
		ready := task6OnlyCall(t, fset, fn, "user.Character.CooldownReady(sneakCooldownKey)", false)
		attempt := task6OnlyCall(t, fset, fn, "actions.Sneak(&actions.UserActor{User: user, Room: room})", false)
		refusal := task6OnlyCall(t, fset, fn, "actions.CostRefusalText(result.Cost)", false)
		failureCooldown := task6OnlyCall(t, fset, fn, "user.Character.TryCooldown", true)
		progression := exactCallPositions(t, fset, fn.Body, "user.Character.CheckSkillProgression", true)
		require.Len(t, progression, 2)
		require.Less(t, int(ready), int(attempt))
		require.Less(t, int(attempt), int(refusal))
		require.Less(t, int(refusal), int(failureCooldown))
		for _, pos := range progression {
			require.Less(t, int(refusal), int(pos))
		}
	})

	t.Run("mob wrapper", func(t *testing.T) {
		path := filepath.Join(actionsDir, "..", "mobcommands", "sneak.go")
		fset, fn := task6FunctionAST(t, path, "Sneak")
		attempt := task6OnlyCall(t, fset, fn, "actions.Sneak(&actions.MobActor{Mob: mob, Room: room})", false)
		progression := task6OnlyCall(t, fset, fn, `mob.Character.OnSkillUse("skullduggery", 0)`, false)
		require.Less(t, int(attempt), int(progression))
		require.False(t, nodeHasCall(fn.Body, "CooldownReady"))
		require.False(t, nodeHasCall(fn.Body, "TryCooldown"))
		require.False(t, nodeHasCall(fn.Body, "CostRefusalText"), "mob refusal must stay silent")
	})

	t.Run("throw command", func(t *testing.T) {
		path := filepath.Join(actionsDir, "..", "usercommands", "throw.go")
		fset, fn := task6FunctionAST(t, path, "Throw")
		item := task6OnlyCall(t, fset, fn, "findThrowItem(user.Character, rest)", false)
		targets := task6OnlyCall(t, fset, fn, "stageThrowTargets(room)", false)
		ready := task6OnlyCall(t, fset, fn, `user.Character.CooldownReady("special-move")`, false)
		admit := task6OnlyCall(t, fset, fn,
			"admitThrowCost(user.Character, float64(cfg.SpecialMoveBaseStaminaCost))", false)
		revalidate := task6OnlyCall(t, fset, fn,
			"revalidateThrowItem(user.Character, matchItem, itemLocation)", false)
		consumeCooldown := task6OnlyCall(t, fset, fn, "user.Character.TryCooldown", true)
		useBackpack := task6OnlyCall(t, fset, fn, "user.Character.UseItem(matchItem)", false)
		useBandolier := task6OnlyCall(t, fset, fn, "user.Character.UseItemFromPotions(matchItem)", false)
		// U6b Task 15: throw resolves per-target through the channel seam
		// (combat.ResolveChannelAttack), not a hand-rolled combat.RunContest.
		// The ordering contract this test guards is unchanged: no contest
		// may run before the cooldown is consumed.
		contests := exactCallPositions(t, fset, fn.Body, "combat.ResolveChannelAttack", true)
		progression := task6OnlyCall(t, fset, fn, "user.Character.OnSkillUse", true)

		require.Less(t, int(item), int(admit))
		require.Less(t, int(targets), int(admit))
		require.Less(t, int(ready), int(admit))
		require.Less(t, int(admit), int(revalidate))
		require.Less(t, int(revalidate), int(consumeCooldown))
		require.Less(t, int(consumeCooldown), int(useBackpack))
		require.Less(t, int(consumeCooldown), int(useBandolier))
		require.NotEmpty(t, contests)
		for _, pos := range contests {
			require.Less(t, int(consumeCooldown), int(pos))
		}
		require.Less(t, int(consumeCooldown), int(progression))
	})
}

// TestSneak_AffordableEmptyRoomSucceeds verifies that a funded actor can hide
// when there is nobody present to contest the attempt.
func TestSneak_AffordableEmptyRoomSucceeds(t *testing.T) {
	char := newTestChar()
	char.Stamina = 100
	char.StaminaMax.Value = 100
	room := newTestRoom()
	actor := newStubActor(char, room)

	result := Sneak(actor)

	assert.False(t, result.AlreadyHidden, "character should not start hidden")
	assert.True(t, result.Success, "should succeed with empty room")
	assert.False(t, result.InCombat, "should not be marked as in combat")
	assert.Equal(t, characters.CostPaid, result.Cost.Status)
	assert.Equal(t, 2, result.Cost.Charged)
	assert.Equal(t, awareness.Hidden, char.Awareness.State())
}

// TestSneak_InCombat verifies that a character in combat cannot sneak.
func TestSneak_InCombat(t *testing.T) {
	char := newTestChar()
	room := newTestRoom()
	actor := newStubActor(char, room)

	// Simulate combat by setting Aggro (any non-nil value indicates combat)
	char.Aggro = &characters.Aggro{
		Type:          characters.DefaultAttack,
		MobInstanceId: 1,
		UserId:        0,
	}

	result := Sneak(actor)

	assert.True(t, result.InCombat, "should detect combat status")
	assert.False(t, result.Success, "should not be successful")
	assert.False(t, result.AlreadyHidden, "should not be already hidden")
	assert.False(t, result.RollHappened, "no roll should happen when in combat")
}

// Known gap: the EmitsLight branches of CalcSneakScore (AW-026 / AW-027) have
// no unit coverage.
//
// Two skip-only placeholders for these were deleted with the rest of the
// permanently skipped tests (review finding 9). The blocker they described is
// real and is kept here: EmitsLight=true needs buff/equipment setup beyond
// what these table tests construct. The behaviour is currently exercised only
// by in-game smoke testing. The AW-024/025 cases above do cover the
// surrounding conditional machinery.
