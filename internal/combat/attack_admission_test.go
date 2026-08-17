package combat

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// pinAutoattackAdmissionBalance makes the real planning, quote, score and
// damage paths arithmetically deterministic:
//
//   - two empty hands plan two swings each at full Stamina;
//   - each swing costs exactly one point;
//   - zero Stamina applies a 0.75 resource multiplier;
//   - rank-50 combat skill contributes 50 to hit score and a 3x damage
//     multiplier.
//
// The production mutations caught by these values are recalculating swings
// after payment, composing the quote twice, stripping skill from damage, or
// applying the skill omission before/after a different resource snapshot.
func pinAutoattackAdmissionBalance(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.AttackBaseStaminaCost = 1
	b.AttackCostModifier = 1
	b.CostSkillMultAtZero = 1
	b.CostSkillMultAtMid = 1
	b.CostSkillMultAtCap = 1
	b.CostSkillMidRank = 25
	b.CostSkillCapRank = 100
	b.CostEncumbranceKnee = 0.75
	b.CostEncumbranceKneeMult = 1
	b.CostEncumbranceMax = 1
	b.CostTotalMultiplierMax = 6
	b.SkillWeight = 1
	b.SkillSoftCap = 50
	b.UnarmedSpeedMultiplier = 0.9
	b.StaminaPenaltyMax = 0.25
	b.ResourcePenaltyCurve = 2
	b.UnarmedDamageMultiplier = 1
	b.SkillMultiplierBase = 1
	b.SkillMultiplierMax = 3
	b.MeleeDamageScale = 0.5
	b.GlobalDamageMultiplier = 1
	b.MobDamageMultiplier = 1
	b.ContestFloor = 0
	cfg.GamePlay.UseSkillProgression = false
	configs.SetConfigForTest(t, cfg)
}

func autoattackAdmissionCharacter(t *testing.T, name string, stamina int) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Name = name
	c.RoomId = 1
	c.Stats.Strength.Base = 100
	c.Stats.Dexterity.Base = 100
	c.Stats.Vitality.Base = 100
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	c.Stats.Vitality.Recalculate()
	if err := c.Validate(); err != nil {
		t.Fatalf("validate %s: %v", name, err)
	}
	c.Skills[string(skills.UnarmedCombat)] = 50
	c.HealthMax.Value = 1000
	c.Health = 1000
	c.StaminaMax.Value = stamina
	c.Stamina = stamina
	c.SetAggro(0, 1, characters.DefaultAttack)
	return c
}

func autoattackAdmissionTarget(t *testing.T) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Name = "target"
	c.RoomId = 1
	c.Stats.Strength.Base = 100
	c.Stats.Dexterity.Base = 100
	c.Stats.Vitality.Base = 100
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	c.Stats.Vitality.Recalculate()
	if err := c.Validate(); err != nil {
		t.Fatalf("validate target: %v", err)
	}
	c.HealthMax.Value = 100000
	c.Health = 100000
	c.StaminaMax.Value = 100000
	c.Stamina = 100000
	return c
}

func countAutoattackShortageMessages(result AttackResult) int {
	count := 0
	for _, msg := range result.MessagesToSource {
		if msg.Category == messaging.CategorySystem && msg.Text == autoattackShortageText {
			count++
		}
	}
	return count
}

// TestAutoattackShortCostPlansBeforePayment catches any production change
// that recalculates the weapon list or swing count after committing Stamina,
// stops when the pool empties, strips skill from damage, applies a pre-payment
// Stamina multiplier, or emits shortage prose per swing.
//
// Literal score derivation, independent of calcAttackScore:
//
//	pre-payment:      (Dex 100 + rank 50 * weight 1) * 1.00 = 150
//	post-payment short: Dex 100 * zero-SP multiplier 0.75 = 75
//
// Literal damage mean: Strength 100 * rank-50 multiplier 3 * unarmed 1 *
// physical scale 0.5 * global 1 = 150. Skill remains in that pipeline.
func TestAutoattackShortCostPlansBeforePaymentAndAttemptsEverySwing(t *testing.T) {
	pinAutoattackAdmissionBalance(t)

	attacker := autoattackAdmissionCharacter(t, "short attacker", 3)
	target := autoattackAdmissionTarget(t)
	ctx := combatContext{sourceCanSee: true, targetCanSee: true, forceCrit: true}

	plan := buildAttackPlan(attacker, target)
	if plan.totalSwings != 4 || len(plan.weapons) != 2 {
		t.Fatalf("pre-payment plan = %d swings across %d weapons, want 4 across 2",
			plan.totalSwings, len(plan.weapons))
	}
	for i, ws := range plan.weapons {
		if ws.swingCount != 2 {
			t.Fatalf("pre-payment weapon %d planned %d swings, want 2", i, ws.swingCount)
		}
	}
	if got := calcAttackScore(attacker, target, 0, ctx); math.Abs(got-150) > 1e-9 {
		t.Fatalf("pre-payment score = %.3f, want literal 150", got)
	}
	if got := buildDamageParams(attacker, target, plan.weapons[0], 0, User).dmgMean; math.Abs(got-150) > 1e-9 {
		t.Fatalf("planned damage mean = %.3f, want literal 150 with rank-50 multiplier intact", got)
	}

	result, cost := resolveCombatRound(attacker, target, User, Mob, ctx)

	if cost.Status != characters.CostPartiallyPaid || !cost.Short() || cost.Charged != 3 {
		t.Fatalf("round cost = %+v, want one partial three-point commit", cost)
	}
	if attacker.Stamina != 0 {
		t.Fatalf("Stamina after short round = %d, want 0 without overdraw", attacker.Stamina)
	}
	if result.SwingsThrown != 4 || len(result.SwingEvents) != 4 {
		t.Fatalf("resolved %d/%d recorded swings, want all 4 planned swings",
			result.SwingsThrown, len(result.SwingEvents))
	}
	if got := countAutoattackShortageMessages(result); got != 1 {
		t.Fatalf("player shortage messages = %d, want exactly one for the round", got)
	}

	shortCtx := ctx
	shortCtx.omitAttackSkill = true
	if got := calcAttackScore(attacker, target, 0, shortCtx); math.Abs(got-75) > 1e-9 {
		t.Fatalf("post-commit short score = %.3f, want literal 75", got)
	}
	if got := buildDamageParams(attacker, target, plan.weapons[0], 0, User).dmgMean; math.Abs(got-150) > 1e-9 {
		t.Fatalf("post-commit short damage mean = %.3f, want skill-driven 150 unchanged", got)
	}

	// If planning moved after payment, this fixture drops from two swings per
	// hand to one. The resolved count above therefore proves calculateCombat
	// consumed the saved four-swing plan rather than asking again at zero SP.
	if recalculated := buildAttackPlan(attacker, target).totalSwings; recalculated != 2 {
		t.Fatalf("fixture guard: post-commit replanning gives %d swings, want 2", recalculated)
	}
}

// TestAutoattackAffordableCostPreservesSkill is the affordable control. It
// catches accidentally stripping the hit skill on CostPaid, changing the
// skill-driven plan, or letting admission alter the damage multiplier.
//
// Both controls have zero Stamina when scored. The only hit-score delta is
// therefore (rank 50 * weight 1) * zero-SP multiplier 0.75 = 37.5.
func TestAutoattackAffordableCostPreservesSkillSwingCountAndDamage(t *testing.T) {
	pinAutoattackAdmissionBalance(t)

	attacker := autoattackAdmissionCharacter(t, "funded attacker", 4)
	target := autoattackAdmissionTarget(t)
	ctx := combatContext{sourceCanSee: true, targetCanSee: true, forceCrit: true}
	plan := buildAttackPlan(attacker, target)
	if plan.totalSwings != 4 {
		t.Fatalf("affordable plan = %d swings, want 4", plan.totalSwings)
	}

	result, cost := resolveCombatRound(attacker, target, User, Mob, ctx)
	if cost.Status != characters.CostPaid || cost.Short() || cost.Charged != 4 {
		t.Fatalf("affordable round cost = %+v, want one full four-point commit", cost)
	}
	if attacker.Stamina != 0 {
		t.Fatalf("Stamina after exactly-affordable round = %d, want 0", attacker.Stamina)
	}
	if result.SwingsThrown != plan.totalSwings || len(result.SwingEvents) != plan.totalSwings {
		t.Fatalf("affordable round attempted %d/%d swings, planned %d",
			result.SwingsThrown, len(result.SwingEvents), plan.totalSwings)
	}
	if got := countAutoattackShortageMessages(result); got != 0 {
		t.Fatalf("affordable round emitted %d shortage messages, want none", got)
	}

	withSkill := calcAttackScore(attacker, target, 0, ctx)
	withoutSkillCtx := ctx
	withoutSkillCtx.omitAttackSkill = true
	withoutSkill := calcAttackScore(attacker, target, 0, withoutSkillCtx)
	if math.Abs(withSkill-112.5) > 1e-9 || math.Abs(withoutSkill-75) > 1e-9 {
		t.Fatalf("post-commit scores with/without skill = %.3f/%.3f, want 112.5/75",
			withSkill, withoutSkill)
	}
	if delta := withSkill - withoutSkill; math.Abs(delta-37.5) > 1e-9 {
		t.Fatalf("skill score delta = %.3f, want literal 37.5 and no other omitted term", delta)
	}
	if got := buildDamageParams(attacker, target, plan.weapons[0], 0, User).dmgMean; math.Abs(got-150) > 1e-9 {
		t.Fatalf("affordable damage mean = %.3f, want literal skill-driven 150", got)
	}
}

// TestAutoattackShortCostPreservesProgressionAndMessagesOnlyPlayers catches a
// short round suppressing the existing progression hook or leaking a private
// player-only explanation into mob combat.
func TestAutoattackShortCostPreservesProgressionAndMessagesOnlyPlayers(t *testing.T) {
	pinAutoattackAdmissionBalance(t)
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default", LitArea: true},
	}))
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		1: {RoomId: 1},
	}, map[string]*rooms.ZoneConfig{}))

	user := users.NewUserRecord(77, 0)
	user.Character = autoattackAdmissionCharacter(t, "player attacker", 3)
	user.Character.SetUserId(user.UserId)
	targetChar := autoattackAdmissionTarget(t)
	targetMob := &mobs.Mob{InstanceId: 1, Character: *targetChar}

	result := AttackPlayerVsMob(user, targetMob, true)
	if got := countAutoattackShortageMessages(result); got != 1 {
		t.Fatalf("player wrapper shortage messages = %d, want one", got)
	}
	if got := user.Character.SkillUseCount[string(skills.UnarmedCombat)]; got != 1 {
		t.Fatalf("short successful round tracked Unarmed Combat %d times, want existing once-per-round hook", got)
	}
	if result.SwingsThrown != 4 {
		t.Fatalf("player wrapper attempted %d swings, want all 4 planned", result.SwingsThrown)
	}

	mobAttacker := autoattackAdmissionCharacter(t, "mob attacker", 3)
	mobTarget := autoattackAdmissionTarget(t)
	mobResult, mobCost := resolveCombatRound(mobAttacker, mobTarget, Mob, Mob, combatContext{
		sourceCanSee: true, targetCanSee: true, forceCrit: true,
	})
	if !mobCost.Short() || mobAttacker.Stamina != 0 || mobResult.SwingsThrown != 4 {
		t.Fatalf("mob short mechanics cost=%+v stamina=%d swings=%d, want player parity",
			mobCost, mobAttacker.Stamina, mobResult.SwingsThrown)
	}
	if got := countAutoattackShortageMessages(mobResult); got != 0 {
		t.Fatalf("mob round emitted %d private player shortage messages, want none", got)
	}
}

func combatSourceFile(t *testing.T, name string) (*token.FileSet, *ast.File) {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(current), name)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return fset, file
}

func combatFuncDecl(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("function %s not found", name)
	return nil
}

func directCallName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	default:
		return ""
	}
}

func callsInBody(body *ast.BlockStmt) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func formattedCombatNode(t *testing.T, fset *token.FileSet, node ast.Node) string {
	t.Helper()
	var out bytes.Buffer
	if err := format.Node(&out, fset, node); err != nil {
		t.Fatalf("format AST node: %v", err)
	}
	return out.String()
}

// TestAttackCostAdmissionStructure catches the realistic recurrence mutations
// that behavioral pool assertions cannot distinguish: moving the charge after
// resolution, charging twice, recalculating a depleted plan, or passing the
// already-composed per-swing result back as ActionCostRequest.Base.
func TestAttackCostAdmissionStructurePlansAndCommitsRawQuoteOnce(t *testing.T) {
	combatFset, combatFile := combatSourceFile(t, "combat.go")
	resolve := combatFuncDecl(t, combatFile, "resolveCombatRound")
	resolveCalls := callsInBody(resolve.Body)
	var chargePositions, calculatePositions []token.Pos
	for _, call := range resolveCalls {
		switch directCallName(call) {
		case "ChargeAttackCost":
			chargePositions = append(chargePositions, call.Pos())
		case "calculateCombat":
			calculatePositions = append(calculatePositions, call.Pos())
		}
	}
	if len(chargePositions) != 1 || len(calculatePositions) != 1 || chargePositions[0] >= calculatePositions[0] {
		t.Fatalf("resolveCombatRound charge/calculate positions = %v/%v; want exactly one charge before one resolution",
			chargePositions, calculatePositions)
	}
	var omitPositions []token.Pos
	ast.Inspect(resolve.Body, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		if formattedCombatNode(t, combatFset, assign.Lhs[0]) == "ctx.omitAttackSkill" &&
			formattedCombatNode(t, combatFset, assign.Rhs[0]) == "costResult.Short()" {
			omitPositions = append(omitPositions, assign.Pos())
		}
		return true
	})
	if len(omitPositions) != 1 || omitPositions[0] <= chargePositions[0] || omitPositions[0] >= calculatePositions[0] {
		t.Fatalf("resolveCombatRound short-skill assignment positions = %v; want costResult.Short exactly once between charge and resolution",
			omitPositions)
	}

	calculate := combatFuncDecl(t, combatFile, "calculateCombat")
	forbidden := map[string]bool{
		"collectAttackWeapons": true,
		"buildWeaponSetup":     true,
		"calcSwingCount":       true,
		"ChargeAttackCost":     true,
		"QuoteActionCost":      true,
		"CommitCost":           true,
	}
	for _, call := range callsInBody(calculate.Body) {
		if name := directCallName(call); forbidden[name] {
			t.Errorf("calculateCombat still calls %s; it must consume the pre-payment plan and never charge", name)
		}
	}

	for _, name := range []string{"AttackPlayerVsMob", "AttackPlayerVsPlayer", "AttackMobVsPlayer", "AttackMobVsMob"} {
		fn := combatFuncDecl(t, combatFile, name)
		resolved := 0
		for _, call := range callsInBody(fn.Body) {
			switch directCallName(call) {
			case "resolveCombatRound":
				resolved++
			case "calculateCombat", "ChargeAttackCost", "QuoteActionCost", "CommitCost":
				t.Errorf("%s bypasses round admission through %s", name, directCallName(call))
			}
		}
		if resolved != 1 {
			t.Errorf("%s calls resolveCombatRound %d times, want exactly once", name, resolved)
		}
	}

	costFset, costFile := combatSourceFile(t, "attack_cost.go")
	charge := combatFuncDecl(t, costFile, "ChargeAttackCost")
	var request *ast.CompositeLit
	commitPartial := 0
	for _, call := range callsInBody(charge.Body) {
		if directCallName(call) == "CommitCost" && len(call.Args) == 2 &&
			formattedCombatNode(t, costFset, call.Args[1]) == "characters.CostPartial" {
			commitPartial++
		}
	}
	ast.Inspect(charge.Body, func(node ast.Node) bool {
		lit, ok := node.(*ast.CompositeLit)
		if !ok || formattedCombatNode(t, costFset, lit.Type) != "characters.ActionCostRequest" {
			return true
		}
		request = lit
		return true
	})
	if request == nil {
		t.Fatal("ChargeAttackCost does not construct a raw characters.ActionCostRequest")
	}
	fields := map[string]string{}
	for _, elt := range request.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		fields[formattedCombatNode(t, costFset, kv.Key)] = formattedCombatNode(t, costFset, kv.Value)
	}
	want := map[string]string{
		"Action":   "costs.ActionAttack",
		"Pool":     "characters.PoolStamina",
		"Base":     "float64(cfg.AttackBaseStaminaCost)",
		"Modifier": "float64(cfg.AttackCostModifier)",
		"Units":    "swings",
	}
	for field, value := range want {
		if fields[field] != value {
			t.Errorf("ActionCostRequest.%s = %q, want raw %q", field, fields[field], value)
		}
	}
	if commitPartial != 1 {
		t.Errorf("ChargeAttackCost partial commits = %d, want exactly one", commitPartial)
	}

	// The test imports costs so a renamed/removed registry constant is a compile
	// failure rather than an AST string silently becoming stale.
	if costs.ActionAttack == "" || strings.TrimSpace(autoattackShortageText) == "" {
		t.Fatal("attack action and shortage text must be non-empty")
	}
}
