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
	"github.com/GoMudEngine/GoMud/internal/items"
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
	if got := calcAttackScore(attacker, target, items.Item{}, 0, ctx); math.Abs(got-150) > 1e-9 {
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
	if got := calcAttackScore(attacker, target, items.Item{}, 0, shortCtx); math.Abs(got-75) > 1e-9 {
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

	withSkill := calcAttackScore(attacker, target, items.Item{}, 0, ctx)
	withoutSkillCtx := ctx
	withoutSkillCtx.omitAttackSkill = true
	withoutSkill := calcAttackScore(attacker, target, items.Item{}, 0, withoutSkillCtx)
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
// short round leaking a private player-only explanation into mob combat.
//
// U9: this test used to also assert that AttackPlayerVsMob itself tracked
// Unarmed Combat skill use once per round. That assertion was pinning the
// phase-2/phase-5 melee progression duplication (see
// internal/hooks/progression_duplication_test.go) — AttackPlayerVsMob no
// longer does any progression tracking at all; that is now the exclusive job
// of hooks.applyCombatProgression (phase 5 of the unified combat
// orchestrator), which this low-level combat-package test does not invoke.
// The assertion was removed rather than updated to a new count because there
// is nothing left in this call path to count.
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

func combatCallArgsMatch(t *testing.T, fset *token.FileSet, call *ast.CallExpr, want []string) bool {
	t.Helper()
	if len(call.Args) != len(want) {
		return false
	}
	for i, arg := range call.Args {
		if formattedCombatNode(t, fset, arg) != want[i] {
			return false
		}
	}
	return true
}

// requireLocalCombatCall accepts only a package-local function call whose Fun
// is the expected *ast.Ident. A selector with the same terminal name is not a
// local call and must fail the guard rather than being collapsed into one.
func requireLocalCombatCall(t *testing.T, fset *token.FileSet, body *ast.BlockStmt, name string, wantArgs ...string) *ast.CallExpr {
	t.Helper()
	var named, exact []*ast.CallExpr
	for _, call := range callsInBody(body) {
		ident, ok := call.Fun.(*ast.Ident)
		if !ok || ident.Name != name {
			continue
		}
		named = append(named, call)
		if combatCallArgsMatch(t, fset, call, wantArgs) {
			exact = append(exact, call)
		}
	}
	if len(named) != 1 || len(exact) != 1 {
		var got []string
		for _, call := range named {
			got = append(got, formattedCombatNode(t, fset, call))
		}
		t.Fatalf("local %s calls = %v; want exactly %s(%s)",
			name, got, name, strings.Join(wantArgs, ", "))
	}
	return exact[0]
}

func requireCombatMethodCall(t *testing.T, fset *token.FileSet, body *ast.BlockStmt, receiver, method string, wantArgs ...string) *ast.CallExpr {
	t.Helper()
	var named, exact []*ast.CallExpr
	for _, call := range callsInBody(body) {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != method {
			continue
		}
		named = append(named, call)
		receiverIdent, ok := sel.X.(*ast.Ident)
		if ok && receiverIdent.Name == receiver && combatCallArgsMatch(t, fset, call, wantArgs) {
			exact = append(exact, call)
		}
	}
	if len(named) != 1 || len(exact) != 1 {
		var got []string
		for _, call := range named {
			got = append(got, formattedCombatNode(t, fset, call))
		}
		t.Fatalf(".%s calls = %v; want exactly %s.%s(%s)",
			method, got, receiver, method, strings.Join(wantArgs, ", "))
	}
	return exact[0]
}

func requireCombatMethodCallArity(t *testing.T, fset *token.FileSet, body *ast.BlockStmt, receiver, method string, arity int) *ast.CallExpr {
	t.Helper()
	var named, exact []*ast.CallExpr
	for _, call := range callsInBody(body) {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != method {
			continue
		}
		named = append(named, call)
		receiverIdent, ok := sel.X.(*ast.Ident)
		if ok && receiverIdent.Name == receiver && len(call.Args) == arity {
			exact = append(exact, call)
		}
	}
	if len(named) != 1 || len(exact) != 1 {
		var got []string
		for _, call := range named {
			got = append(got, formattedCombatNode(t, fset, call))
		}
		t.Fatalf(".%s calls = %v; want exactly one %s.%s call with %d arguments",
			method, got, receiver, method, arity)
	}
	return exact[0]
}

func requireCombatCallAssignment(t *testing.T, body *ast.BlockStmt, call *ast.CallExpr, name string) {
	t.Helper()
	matches := 0
	ast.Inspect(body, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) == 0 || len(assign.Rhs) != 1 || assign.Rhs[0] != call {
			return true
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if ok && ident.Name == name {
			matches++
		}
		return true
	})
	if matches != 1 {
		t.Fatalf("call assignment to %s matches = %d, want exactly one := assignment", name, matches)
	}
}

func requireShortSkillAssignment(t *testing.T, body *ast.BlockStmt) token.Pos {
	t.Helper()
	var positions []token.Pos
	ast.Inspect(body, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		lhs, ok := assign.Lhs[0].(*ast.SelectorExpr)
		if !ok || lhs.Sel.Name != "omitAttackSkill" {
			return true
		}
		lhsReceiver, ok := lhs.X.(*ast.Ident)
		if !ok || lhsReceiver.Name != "ctx" {
			return true
		}
		rhs, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || len(rhs.Args) != 0 {
			return true
		}
		method, ok := rhs.Fun.(*ast.SelectorExpr)
		if !ok || method.Sel.Name != "Short" {
			return true
		}
		rhsReceiver, ok := method.X.(*ast.Ident)
		if ok && rhsReceiver.Name == "costResult" {
			positions = append(positions, assign.Pos())
		}
		return true
	})
	if len(positions) != 1 {
		t.Fatalf("ctx.omitAttackSkill = costResult.Short() assignments = %v, want exactly one", positions)
	}
	return positions[0]
}

// TestAttackCostAdmissionStructure catches the realistic recurrence mutations
// that behavioral pool assertions cannot distinguish: moving the charge after
// resolution, dropping either ctx propagation edge, charging twice, routing a
// wrapper's actors incorrectly, recalculating a depleted plan, using a different
// commit receiver/policy, or passing a composed per-swing result back as Base.
func TestAttackCostAdmissionStructurePlansAndCommitsRawQuoteOnce(t *testing.T) {
	combatFset, combatFile := combatSourceFile(t, "combat.go")
	resolve := combatFuncDecl(t, combatFile, "resolveCombatRound")
	planCall := requireLocalCombatCall(t, combatFset, resolve.Body, "buildAttackPlan",
		"sourceChar", "targetChar")
	requireCombatCallAssignment(t, resolve.Body, planCall, "plan")
	chargeCall := requireLocalCombatCall(t, combatFset, resolve.Body, "ChargeAttackCost",
		"sourceChar", "plan.totalSwings")
	requireCombatCallAssignment(t, resolve.Body, chargeCall, "costResult")
	omitPosition := requireShortSkillAssignment(t, resolve.Body)
	calculateCall := requireLocalCombatCall(t, combatFset, resolve.Body, "calculateCombat",
		"sourceChar", "targetChar", "sourceType", "targetType", "plan", "ctx")
	requireCombatCallAssignment(t, resolve.Body, calculateCall, "attackResult")
	if !(planCall.Pos() < chargeCall.Pos() && chargeCall.Pos() < omitPosition && omitPosition < calculateCall.Pos()) {
		t.Fatalf("resolveCombatRound plan/charge/omit/calculate positions = %d/%d/%d/%d; want strict pre-resolution order",
			planCall.Pos(), chargeCall.Pos(), omitPosition, calculateCall.Pos())
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
		var name string
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			name = fn.Name
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		}
		if forbidden[name] {
			t.Errorf("calculateCombat still calls %s; it must consume the pre-payment plan and never charge", name)
		}
	}
	// ws.weapon, not the main-hand weapon. calcAttackScore resolves the attack
	// SKILL from whatever it is handed, and calculateCombat swings every weapon
	// in the plan through this one call. Passing anything that is not the weapon
	// being swung reintroduces the offhand-fist bug: punch alongside a sword and
	// the punch is scored at your weapon-combat rank.
	requireLocalCombatCall(t, combatFset, calculate.Body, "calcAttackScore",
		"sourceChar", "targetChar", "ws.weapon", "ws.penalty", "ctx")

	wrapperCalls := []struct {
		name string
		args []string
	}{
		{"AttackPlayerVsMob", []string{"user.Character", "&mob.Character", "User", "Mob", "ctx"}},
		{"AttackPlayerVsPlayer", []string{"userAtk.Character", "userDef.Character", "User", "User", "ctx"}},
		{"AttackMobVsPlayer", []string{"&mob.Character", "user.Character", "Mob", "User", "ctx"}},
		{"AttackMobVsMob", []string{"&mobAtk.Character", "&mobDef.Character", "Mob", "Mob", "ctx"}},
	}
	for _, wrapper := range wrapperCalls {
		fn := combatFuncDecl(t, combatFile, wrapper.name)
		requireLocalCombatCall(t, combatFset, fn.Body, "resolveCombatRound", wrapper.args...)
		for _, call := range callsInBody(fn.Body) {
			var bypass string
			switch called := call.Fun.(type) {
			case *ast.Ident:
				if called.Name != "resolveCombatRound" && forbidden[called.Name] {
					bypass = called.Name
				}
			case *ast.SelectorExpr:
				if forbidden[called.Sel.Name] {
					bypass = called.Sel.Name
				}
			}
			if bypass != "" {
				t.Errorf("%s bypasses round admission through %s", wrapper.name, bypass)
			}
		}
	}

	costFset, costFile := combatSourceFile(t, "attack_cost.go")
	charge := combatFuncDecl(t, costFile, "ChargeAttackCost")
	quoteCall := requireCombatMethodCallArity(t, costFset, charge.Body, "attacker", "QuoteActionCost", 1)
	requireCombatCallAssignment(t, charge.Body, quoteCall, "quote")
	request, ok := quoteCall.Args[0].(*ast.CompositeLit)
	if !ok || formattedCombatNode(t, costFset, request.Type) != "characters.ActionCostRequest" {
		t.Fatalf("QuoteActionCost argument = %T, want characters.ActionCostRequest literal", quoteCall.Args[0])
	}
	fields := map[string]string{}
	for _, elt := range request.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			t.Fatalf("ActionCostRequest contains non-keyed element %s", formattedCombatNode(t, costFset, elt))
		}
		field := formattedCombatNode(t, costFset, kv.Key)
		if _, duplicate := fields[field]; duplicate {
			t.Fatalf("ActionCostRequest contains duplicate field %s", field)
		}
		fields[field] = formattedCombatNode(t, costFset, kv.Value)
	}
	want := map[string]string{
		"Action":   "costs.ActionAttack",
		"Pool":     "characters.PoolStamina",
		"Base":     "float64(cfg.AttackBaseStaminaCost)",
		"Modifier": "float64(cfg.AttackCostModifier)",
		"Units":    "swings",
	}
	if len(fields) != len(want) {
		t.Errorf("ActionCostRequest has %d fields, want exactly %d: %v", len(fields), len(want), fields)
	}
	for field, value := range want {
		if fields[field] != value {
			t.Errorf("ActionCostRequest.%s = %q, want raw %q", field, fields[field], value)
		}
	}
	requireCombatMethodCall(t, costFset, charge.Body, "attacker", "CommitCost",
		"quote", "characters.CostPartial")

	// The test imports costs so a renamed/removed registry constant is a compile
	// failure rather than an AST string silently becoming stale.
	if costs.ActionAttack == "" || strings.TrimSpace(autoattackShortageText) == "" {
		t.Fatal("attack action and shortage text must be non-empty")
	}
}
