package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

func pinDefenceAdmissionConfig(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.SkillWeight = 2
	cfg.Balance.DefenceBaseStaminaCost = 10
	cfg.Balance.DodgeCostModifier = 1.25
	cfg.Balance.ParryCostModifier = 1.10
	cfg.Balance.BlockCostModifier = 1.15
	cfg.Balance.QuellBaseConvictionCost = 20
	cfg.Balance.DefyBaseConvictionCost = 30
	cfg.Balance.CostSkillMultAtZero = 1
	cfg.Balance.CostSkillMultAtMid = 1
	cfg.Balance.CostSkillMultAtCap = 1
	cfg.Balance.CostSkillMidRank = 25
	cfg.Balance.CostSkillCapRank = 100
	cfg.Balance.CostTotalMultiplierMax = 100
	cfg.Balance.DodgeEffectiveness = 1
	cfg.Balance.ParryEffectiveness = 1
	cfg.Balance.BlockEffectiveness = 1
	cfg.Balance.QuellEffectiveness = 1
	cfg.Balance.DefyEffectiveness = 1
	cfg.Balance.MinAttackCritChance = 0
	cfg.Balance.MinDefenseCritChance = 0
	cfg.Balance.ProneDodgePenalty = 1
	cfg.Balance.ProneParryPenalty = 1
	cfg.Balance.ProneBlockPenalty = 1
	cfg.Balance.ClinchDodgePenalty = 1
	cfg.Balance.ClinchParryPenalty = 1
	cfg.Balance.ClinchBlockPenalty = 1
	cfg.Balance.GroundedDodgePenalty = 1
	cfg.Balance.GroundedParryPenalty = 1
	cfg.Balance.GroundedBlockPenalty = 1
	cfg.Balance.ThirdPartyGrapplePenalty = 1
	cfg.Balance.DarknessCombatPenalty = 1
	configs.SetConfigForTest(t, cfg)
}

func defenceAdmissionCharacters() (*characters.Character, *characters.Character) {
	attacker := characters.New()
	attacker.Stats.Strength.Base = 100
	attacker.Stats.Strength.Recalculate()
	attacker.Stats.Dexterity.Base = 100
	attacker.Stats.Dexterity.Recalculate()

	defender := characters.New()
	defender.Stats.Strength.Base = 100
	defender.Stats.Strength.Recalculate()
	defender.Stats.Dexterity.Base = 100
	defender.Stats.Dexterity.Recalculate()
	defender.Stats.Willpower.Base = 100
	defender.Stats.Willpower.Recalculate()
	defender.SetSkill(string(skills.UnarmedCombat), 20)
	defender.SetSkill(string(skills.WeaponCombat), 30)
	defender.SetSkill(string(skills.Spellcasting), 20)
	defender.SetSkill(string(skills.Rhetoric), 30)
	return attacker, defender
}

func deterministicDefenceResult(winner string, success, floored bool, margin, defZ, stdDev float64) contest.Result {
	return contest.Result{
		AttackRoll:  dice.RollResult{Value: 100, Mean: 100, StdDev: stdDev, ZScore: 0},
		DefenseRoll: dice.RollResult{Value: 100 - margin, Mean: 100, StdDev: stdDev, ZScore: defZ},
		Margin:      margin,
		Winner:      winner,
		Contested:   true,
		Success:     success,
		Floored:     floored,
	}
}

func countDefenceShortageMessages(result *AttackResult) int {
	count := 0
	for _, msg := range result.MessagesToTarget {
		if msg.Category == messaging.CategorySystem && msg.Text == defenceShortageText {
			count++
		}
	}
	return count
}

// Mutation proof: dodge is first and short, while parry is second, affordable,
// and selected. Committing by entry index instead of winner name turns the paid
// result into a partial one, drains the same pool through the wrong quote, emits
// shortage copy, and progresses the wrong skill.
func TestRunBestOfAllDefense_MixedAffordabilityPairsWinnerWithItsOwnQuote(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Stamina = 11 // dodge owes 12; parry owes 11
	defender.SetUserId(91)

	result := &AttackResult{}
	runnerCalls := 0
	runner := func(atkScore float64, entries []contest.Entry) contest.Result {
		runnerCalls++
		if atkScore != 100 {
			t.Fatalf("attack score = %.2f, want 100", atkScore)
		}
		if len(entries) != 2 {
			t.Fatalf("eligible entries = %d, want 2", len(entries))
		}
		if entries[0].Name != characters.DefenseDodge || entries[0].Score != 100 {
			t.Fatalf("short dodge entry = %+v, want name=dodge score=100 without skill", entries[0])
		}
		if entries[1].Name != characters.DefenseParry || entries[1].Score != 160 {
			t.Fatalf("affordable parry entry = %+v, want name=parry score=160 with skill", entries[1])
		}
		return deterministicDefenceResult(characters.DefenseParry, false, false, -10, 1, 10)
	}

	best := runBestOfAllDefenseWithRunner(result, attacker, defender,
		[]string{characters.DefenseDodge, characters.DefenseParry}, 100, false,
		combatContext{sourceCanSee: true, targetCanSee: true}, runner)

	if runnerCalls != 1 {
		t.Fatalf("runner calls = %d, want 1", runnerCalls)
	}
	if best.defenseType != characters.DefenseParry {
		t.Fatalf("winner = %q, want parry", best.defenseType)
	}
	if best.cost.Status != characters.CostPaid || best.cost.Charged != 11 || best.cost.Short() {
		t.Fatalf("winner cost = %+v, want paid 11 stamina", best.cost)
	}
	if defender.Stamina != 0 {
		t.Fatalf("stamina = %d, want 0 after only the 11-point parry quote commits", defender.Stamina)
	}
	if got := countDefenceShortageMessages(result); got != 0 {
		t.Fatalf("shortage messages = %d, want 0; the losing short dodge is silent", got)
	}

	resolveDefenseOutcome(result, best, attacker, defender, ContestCritThreshold, false, false)
	if got := defender.SkillUseCount[string(skills.UnarmedCombat)]; got != 0 {
		t.Fatalf("losing dodge progression = %d, want 0", got)
	}
	if got := defender.SkillUseCount[string(skills.WeaponCombat)]; got != 1 {
		t.Fatalf("winning parry progression = %d, want 1", got)
	}
}

func TestRunBestOfAllDefense_ShortWinnerChargesMessagesAndProgressesOnce(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Stamina = 5
	defender.SetUserId(92)
	result := &AttackResult{}

	runner := func(_ float64, entries []contest.Entry) contest.Result {
		if len(entries) != 2 || entries[0].Score != 100 || entries[1].Score != 100 {
			t.Fatalf("short candidates = %+v, want dodge/parry both at stat-only score 100", entries)
		}
		return deterministicDefenceResult(characters.DefenseDodge, false, false, -10, 1, 10)
	}

	best := runBestOfAllDefenseWithRunner(result, attacker, defender,
		[]string{characters.DefenseDodge, characters.DefenseParry}, 100, false,
		combatContext{sourceCanSee: true, targetCanSee: true}, runner)
	if best.cost.Status != characters.CostPartiallyPaid || best.cost.Charged != 5 || !best.cost.Short() {
		t.Fatalf("short winner cost = %+v, want partially paid 5", best.cost)
	}
	if defender.Stamina != 0 {
		t.Fatalf("stamina = %d, want 0 without going negative", defender.Stamina)
	}

	// A later swing in the same AttackResult may also select a short winner, but
	// the private player explanation belongs to the round and appears once.
	second := runBestOfAllDefenseWithRunner(result, attacker, defender,
		[]string{characters.DefenseDodge, characters.DefenseParry}, 100, false,
		combatContext{sourceCanSee: true, targetCanSee: true}, runner)
	if second.cost.Status != characters.CostPartiallyPaid || second.cost.Charged != 0 {
		t.Fatalf("second short winner cost = %+v, want partial zero from empty pool", second.cost)
	}
	if got := countDefenceShortageMessages(result); got != 1 {
		t.Fatalf("shortage messages = %d, want exactly 1 for the round", got)
	}

	resolveDefenseOutcome(result, best, attacker, defender, ContestCritThreshold, false, false)
	if got := defender.SkillUseCount[string(skills.UnarmedCombat)]; got != 1 {
		t.Fatalf("short winning dodge progression = %d, want 1 under existing policy", got)
	}
	if got := defender.SkillUseCount[string(skills.WeaponCombat)]; got != 0 {
		t.Fatalf("losing parry progression = %d, want 0", got)
	}
}

func TestRunBestOfAllDefense_SelectedWinnerPaysWhenAttackWins(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Stamina = 11
	result := &AttackResult{}

	best := runBestOfAllDefenseWithRunner(result, attacker, defender,
		[]string{characters.DefenseParry}, 100, false,
		combatContext{sourceCanSee: true, targetCanSee: true},
		func(_ float64, _ []contest.Entry) contest.Result {
			return deterministicDefenceResult(characters.DefenseParry, true, false, 5, -0.5, 10)
		})

	if best.cost.Status != characters.CostPaid || best.cost.Charged != 11 || defender.Stamina != 0 {
		t.Fatalf("attack-win selected defence cost = %+v stamina=%d, want paid 11 and empty pool",
			best.cost, defender.Stamina)
	}
	if got := defender.SkillUseCount[string(skills.WeaponCombat)]; got != 0 {
		t.Fatalf("progression before outcome narration = %d, want 0", got)
	}
}

func TestRunBestOfAllDefense_ShortScoreRetainsNonSkillMultipliers(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	cfg := configs.GetConfig()
	cfg.Balance.DodgeEffectiveness = 0.5
	cfg.Balance.DarknessCombatPenalty = 0.5
	configs.SetConfigForTest(t, cfg)

	attacker, defender := defenceAdmissionCharacters()
	defender.Stamina = 0
	result := &AttackResult{}

	runBestOfAllDefenseWithRunner(result, attacker, defender,
		[]string{characters.DefenseDodge}, 100, false,
		combatContext{sourceCanSee: true, targetCanSee: false},
		func(_ float64, entries []contest.Entry) contest.Result {
			// Base Dexterity 100 remains. Only Unarmed Combat is omitted, then
			// effectiveness 0.5 and darkness 0.5 both still apply: 100/4 = 25.
			if len(entries) != 1 || entries[0].Score != 25 {
				t.Fatalf("short modified dodge = %+v, want stat-only score 25 after both multipliers", entries)
			}
			return deterministicDefenceResult(characters.DefenseDodge, true, false, 5, 0, 10)
		})
}

func TestRunBestOfAllDefense_ShortNPCWinnerGetsNoPrivateMessage(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Stamina = 0
	result := &AttackResult{}

	best := runBestOfAllDefenseWithRunner(result, attacker, defender,
		[]string{characters.DefenseDodge}, 100, false,
		combatContext{sourceCanSee: true, targetCanSee: true},
		func(_ float64, _ []contest.Entry) contest.Result {
			return deterministicDefenceResult(characters.DefenseDodge, true, false, 5, 0, 10)
		})

	if !best.cost.Short() {
		t.Fatalf("NPC winner cost = %+v, want short", best.cost)
	}
	if got := countDefenceShortageMessages(result); got != 0 {
		t.Fatalf("NPC private shortage messages = %d, want 0", got)
	}
}

func TestResolveChannelDefence_MixedAffordabilityCommitsAndProgressesOnlyWinner(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Stamina = 11 // dodge short at 12; block affordable at 11

	out := resolveChannelDefenceWithRunner(ChannelSpellPhysical, attacker, defender,
		func(_ float64, entries []contest.Entry) contest.Result {
			if len(entries) != 2 {
				t.Fatalf("eligible entries = %d, want 2", len(entries))
			}
			if entries[0].Name != characters.DefenseDodge || entries[0].Score != 100 {
				t.Fatalf("short dodge entry = %+v, want stat-only 100", entries[0])
			}
			if entries[1].Name != characters.DefenseBlock || entries[1].Score != 160 {
				t.Fatalf("affordable block entry = %+v, want full 160", entries[1])
			}
			return deterministicDefenceResult(characters.DefenseBlock, true, false, 5, 1.5, 10)
		})

	if out.DefenceType != characters.DefenseBlock || out.Cost.Status != characters.CostPaid || out.Cost.Charged != 11 {
		t.Fatalf("channel winner/cost = type %q cost %+v, want block paid 11", out.DefenceType, out.Cost)
	}
	if out.DamageMultiplier != 1 || out.Defended || out.DefensiveCrit || out.NormalizedDefenceMargin != 0 {
		t.Fatalf("attack-win outcome = %+v, want full damage and zero defensive outcome", out)
	}
	if out.DefenseRollZScore != 1.5 {
		t.Fatalf("defender self-relative z = %.4f, want retained attack-win roll z 1.5", out.DefenseRollZScore)
	}
	if got := defender.SkillUseCount[string(skills.UnarmedCombat)]; got != 0 {
		t.Fatalf("losing dodge progression = %d, want 0", got)
	}
	if got := defender.SkillUseCount[string(skills.WeaponCombat)]; got != 1 {
		t.Fatalf("winning block progression = %d, want 1", got)
	}
}

func TestResolveChannelDefence_ShortWinnerUsesConvictionAndOmitsOnlySkill(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Conviction = 5

	out := resolveChannelDefenceWithRunner(ChannelSocial, attacker, defender,
		func(_ float64, entries []contest.Entry) contest.Result {
			if len(entries) != 1 || entries[0].Name != characters.DefenseDefy || entries[0].Score != 100 {
				t.Fatalf("short defy entry = %+v, want Willpower-only score 100", entries)
			}
			return deterministicDefenceResult(characters.DefenseDefy, false, false, -10, 0.5, 10)
		})

	if out.Cost.Status != characters.CostPartiallyPaid || out.Cost.Pool != characters.PoolConviction ||
		out.Cost.Charged != 5 || !out.Cost.Short() || defender.Conviction != 0 {
		t.Fatalf("short defy cost = %+v conviction=%d, want partial 5 from conviction", out.Cost, defender.Conviction)
	}
	if got := defender.SkillUseCount[string(skills.Rhetoric)]; got != 1 {
		t.Fatalf("short winning defy progression = %d, want 1 under existing policy", got)
	}
	if !out.Defended || out.DamageMultiplier >= 0.5 {
		t.Fatalf("short defy outcome = %+v, want preserved noncritical defence curve", out)
	}
}

func TestResolveChannelDefence_ReportsOpposedMarginDistinctFromRollZScore(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Conviction = 100

	out := resolveChannelDefenceWithRunner(ChannelSpellMental, attacker, defender,
		func(_ float64, entries []contest.Entry) contest.Result {
			if len(entries) != 1 || entries[0].Name != characters.DefenseQuell || entries[0].Score != 140 {
				t.Fatalf("quell entries = %+v, want one affordable full-score quell at 140", entries)
			}
			return deterministicDefenceResult(characters.DefenseQuell, false, false, -10, 1.75, 10)
		})

	wantMargin := 1 / math.Sqrt2
	wantMultiplier := 1 - (0.5 + 0.5*(wantMargin/ContestCritThreshold))
	if math.Abs(out.NormalizedDefenceMargin-wantMargin) > 1e-12 {
		t.Fatalf("normalized opposed margin = %.12f, want %.12f", out.NormalizedDefenceMargin, wantMargin)
	}
	if out.DefenseRollZScore != 1.75 {
		t.Fatalf("defender roll z = %.4f, want independent self-relative z 1.75", out.DefenseRollZScore)
	}
	if math.Abs(out.DamageMultiplier-wantMultiplier) > 1e-12 {
		t.Fatalf("damage multiplier = %.12f, want %.12f from opposed margin", out.DamageMultiplier, wantMultiplier)
	}
	if out.DefenceType != characters.DefenseQuell || !out.Defended || out.DefensiveCrit {
		t.Fatalf("structured outcome = %+v, want noncritical defended quell", out)
	}
	if out.Cost.Status != characters.CostPaid || out.Cost.Pool != characters.PoolConviction || out.Cost.Charged != 20 {
		t.Fatalf("quell cost = %+v, want paid 20 conviction", out.Cost)
	}
}

func TestResolveChannelDefence_FlooredSaveUsesBareWinSentinels(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Conviction = 100

	out := resolveChannelDefenceWithRunner(ChannelSpellMental, attacker, defender,
		func(_ float64, _ []contest.Entry) contest.Result {
			return deterministicDefenceResult(characters.DefenseQuell, false, true, -1, 9.5, 10)
		})

	if out.DamageMultiplier != 0.5 || !out.Defended || out.DefensiveCrit {
		t.Fatalf("floored save outcome = %+v, want bare defended noncrit at 0.5", out)
	}
	if out.DefenseRollZScore != 0 || out.NormalizedDefenceMargin != 0 {
		t.Fatalf("floored save sentinels = z %.4f margin %.4f, want both zero", out.DefenseRollZScore, out.NormalizedDefenceMargin)
	}
	if out.DefenceType != characters.DefenseQuell || out.Cost.Status != characters.CostPaid {
		t.Fatalf("floored winner/cost = type %q cost %+v, want paid quell", out.DefenceType, out.Cost)
	}
}

func TestResolveChannelDefence_DefensiveCritReportsFullNegation(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Conviction = 100

	out := resolveChannelDefenceWithRunner(ChannelSocial, attacker, defender,
		func(_ float64, _ []contest.Entry) contest.Result {
			// 30 / (10*sqrt(2)) = 2.1213, independently above the 2.0
			// defensive-crit threshold.
			return deterministicDefenceResult(characters.DefenseDefy, false, false, -30, 0.25, 10)
		})

	if out.DamageMultiplier != 0 || !out.Defended || !out.DefensiveCrit {
		t.Fatalf("decisive defence = %+v, want defended crit with full negation", out)
	}
	wantMargin := 3 / math.Sqrt2
	if math.Abs(out.NormalizedDefenceMargin-wantMargin) > 1e-12 || out.DefenseRollZScore != 0.25 {
		t.Fatalf("crit statistics = margin %.12f z %.4f, want %.12f and 0.25",
			out.NormalizedDefenceMargin, out.DefenseRollZScore, wantMargin)
	}
	if out.DefenceType != characters.DefenseDefy || out.Cost.Pool != characters.PoolConviction || out.Cost.Charged != 30 {
		t.Fatalf("defy identity/cost = type %q cost %+v, want defy charged from conviction", out.DefenceType, out.Cost)
	}
}

func TestResolveChannelDefence_NonpositiveStdDevHasZeroNormalizedMargin(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Conviction = 100

	out := resolveChannelDefenceWithRunner(ChannelSpellMental, attacker, defender,
		func(_ float64, _ []contest.Entry) contest.Result {
			return deterministicDefenceResult(characters.DefenseQuell, false, false, -10, 0, 0)
		})
	if out.NormalizedDefenceMargin != 0 || out.DamageMultiplier != 0.5 || !out.Defended || out.DefensiveCrit {
		t.Fatalf("zero-spread outcome = %+v, want zero normalized margin and bare noncrit save", out)
	}
}

func TestResolveChannelDefence_UncontestedUsesZeroSentinels(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	called := false
	out := resolveChannelDefenceWithRunner(AttackChannel("unknown"), attacker, defender,
		func(_ float64, _ []contest.Entry) contest.Result {
			called = true
			return contest.Result{}
		})
	if called {
		t.Fatal("runner called for channel with no eligible defences")
	}
	if out.DamageMultiplier != 1 || out.DefenceType != "" || out.DefenseRollZScore != 0 ||
		out.NormalizedDefenceMargin != 0 || out.Defended || out.DefensiveCrit || out.Cost.Charged != 0 {
		t.Fatalf("uncontested outcome = %+v, want full damage with zero sentinels", out)
	}
}

func TestResolveChannelDefence_InjectedUncontestedResultUsesFullDamage(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Conviction = 100

	out := resolveChannelDefenceWithRunner(ChannelSpellMental, attacker, defender,
		func(_ float64, entries []contest.Entry) contest.Result {
			if len(entries) != 1 {
				t.Fatalf("entries = %d, want one quoted quell before runner", len(entries))
			}
			return contest.Result{
				AttackRoll: dice.RollResult{Value: 100, Mean: 100, StdDev: 10},
			}
		})

	if out.DamageMultiplier != 1 || out.DefenceType != "" || out.Defended || out.DefensiveCrit ||
		out.DefenseRollZScore != 0 || out.NormalizedDefenceMargin != 0 || out.Cost.Charged != 0 {
		t.Fatalf("injected uncontested outcome = %+v, want full damage and zero sentinels", out)
	}
	if defender.Conviction != 100 {
		t.Fatalf("uncontested conviction = %d, want unchanged 100", defender.Conviction)
	}
	if got := defender.SkillUseCount[string(skills.Spellcasting)]; got != 0 {
		t.Fatalf("uncontested progression = %d, want 0", got)
	}
}
