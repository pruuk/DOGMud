package combat

// U10d Task 3 — the DAMAGE half of the surprise-attack redesign.
//
// Exactly ONE swing per engagement is the "opening strike". It contests
// normally (Task 2 supplies the crit-on-clean-win verdict); this file pins what
// that swing is WORTH and, critically, that only one swing per round gets it.
//
// dice.RollStat is stochastic, so every magnitude assertion here is on a
// SAMPLED MEAN, never on a single rolled value.

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// meanOver samples f n times and returns the arithmetic mean.
func meanOver(n int, f func() int) float64 {
	total := 0
	for i := 0; i < n; i++ {
		total += f()
	}
	return float64(total) / float64(n)
}

func TestOpeningStrike_StacksOnceThenStops(t *testing.T) {
	sdp := swingDamageParams{rawDmgForCrit: 100, critDmgMult: 4.0, openingStrikeMult: 3.0}

	_, remaining := calcHitDamage(&AttackResult{}, true, true, sdp)
	if remaining {
		t.Fatalf("the opening strike must be consumed by the swing that lands it")
	}

	stacked := meanOver(2000, func() int { d, _ := calcHitDamage(&AttackResult{}, true, true, sdp); return d })
	plain := meanOver(2000, func() int { d, _ := calcHitDamage(&AttackResult{}, true, false, sdp); return d })

	if ratio := stacked / plain; ratio < 2.7 || ratio > 3.3 {
		t.Fatalf("stacked/plain mean ratio %.2f, want ~3.0 (openingStrikeMult)", ratio)
	}
}

// THE REGRESSION TEST for the retired every-swing design.
func TestOpeningStrike_LaterCritsAreOrdinary(t *testing.T) {
	sdp := swingDamageParams{rawDmgForCrit: 100, critDmgMult: 4.0, openingStrikeMult: 3.0}
	plain := meanOver(2000, func() int { d, _ := calcHitDamage(&AttackResult{}, true, false, sdp); return d })
	if plain < 350 || plain > 450 {
		t.Fatalf("an ordinary crit must roll around rawDmgForCrit*critDmgMult=400, got %.0f", plain)
	}
}

func TestOpeningStrike_DefendedSwingDoesNotPayTheStack(t *testing.T) {
	sdp := swingDamageParams{rawDmgForCrit: 100, critDmgMult: 4.0, openingStrikeMult: 3.0}

	// isCrit false is what a deflected hit carries (Task 2's guard excludes
	// res.defended), so the crit branch must NOT be entered by openingStrike
	// alone -- otherwise a defended opener rolls the full stacked mean and only
	// then gets scaled by damageMult, delivering roughly half a maximum ambush
	// on a swing the defender won.
	dmg, _ := calcHitDamage(&AttackResult{}, false, true, sdp)
	if dmg > 200 {
		t.Fatalf("a defended swing rolled %d — it took the stacked crit branch", dmg)
	}
}

// ─── the round-level contract ───────────────────────────────────────────────
//
// The three tests above call calcHitDamage directly, so they cannot see the
// scope of the flag in production. That scope IS the design: the opening strike
// is one swing per engagement, not one per weapon and not one per swing. This
// test drives real multi-swing rounds through resolveCombatRound and asserts it.
//
// It is built to FAIL if the per-swing local at the resolveDefenseOutcome call
// site is replaced by the round-scoped flag (verified by mutation, both
// directions, when it was written).

// openingStrikeSkullduggery is the attacker's ambush rank, pinned so the
// expected opening-strike multiplier is exact.
const openingStrikeSkullduggery = 20

// openingStrikeCombatant builds one bare-handed stat-100 rank-30 combatant with
// pools deep enough that the per-swing resource multipliers stay at 1.0.
func openingStrikeCombatant(t *testing.T, name string) *characters.Character {
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
	c.Skills[string(skills.UnarmedCombat)] = 30
	// Pinned explicitly: characters.New() seeds skills at rank 1, so leaving
	// this unset would make the expected multiplier depend on that seed.
	c.Skills[string(skills.Skullduggery)] = openingStrikeSkullduggery
	c.HealthMax.Value = 1000000
	c.Health = 1000000
	c.StaminaMax.Value = 1000000
	c.Stamina = 1000000
	return c
}

// pinOpeningStrikeBalance pins every knob this test reads to a known value, so
// a config default drifting cannot quietly move the separation the test relies
// on. SetConfigForTest restores on cleanup.
func pinOpeningStrikeBalance(t *testing.T, surpriseKnob float64) {
	t.Helper()
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.SkillWeight = 2.0
	b.ContestFloor = 0.125
	b.DodgeEffectiveness = 1.0
	b.CritBarSkillSlope = 0.05
	b.CritBarFloor = 1.5
	b.CritBarCeiling = 3.0
	b.MinAttackCritChance = 0.01
	b.MinDefenseCritChance = 0.01
	b.CritDamageBase = 2.0
	b.CritDamagePerSkill = 0.05
	b.UnarmedDamageMultiplier = 0.30
	b.MeleeDamageScale = 0.30
	b.GlobalDamageMultiplier = 1.0
	b.MobDamageMultiplier = 1.0
	b.SkillMultiplierBase = 1.0
	b.SkillMultiplierMax = 3.0
	b.SkillSoftCap = 50
	b.PhysicalMitigationCap = 0.75
	b.StaminaPenaltyMax = 0.28
	b.HealthPenaltyMax = 0.28
	b.ResourcePenaltyCurve = 2.0
	b.SurpriseOpeningStrikeMultiplier = configs.ConfigFloat(surpriseKnob)
	// Live progression would move the ranks mid-run and slide the scores.
	cfg.GamePlay.UseSkillProgression = false
	configs.SetConfigForTest(t, cfg)
}

// runOpeningStrikeRounds resolves n rounds and returns every swing's damage,
// grouped per round in resolution order.
func runOpeningStrikeRounds(t *testing.T, n int, surprise bool) [][]int {
	t.Helper()
	attacker := openingStrikeCombatant(t, "Ambusher")
	defender := openingStrikeCombatant(t, "Mark")
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	rounds := make([][]int, 0, n)
	for i := 0; i < n; i++ {
		attacker.Stamina = attacker.StaminaMax.Value
		attacker.Health = attacker.HealthMax.Value
		defender.Stamina = defender.StaminaMax.Value
		defender.Health = defender.HealthMax.Value
		// A double fumble knocks both prone; reset so every round samples the
		// same standing matchup.
		if !attacker.Position.IsStanding() {
			attacker.Position = position.NewMachine()
		}
		if !defender.Position.IsStanding() {
			defender.Position = position.NewMachine()
		}
		aggro := characters.DefaultAttack
		if surprise {
			aggro = characters.SurpriseAttack
		}
		attacker.SetAggro(0, 1, aggro)
		if attacker.Aggro == nil || attacker.Aggro.Type != aggro {
			t.Fatalf("round %d: SetAggro did not take; got %+v", i, attacker.Aggro)
		}

		result, cost := resolveCombatRound(attacker, defender, User, Mob, ctx)
		if cost.Short() {
			t.Fatalf("round %d: admission ran short — fixture pools are wrong", i)
		}
		swings := make([]int, 0, len(result.SwingEvents))
		for _, ev := range result.SwingEvents {
			swings = append(swings, ev.Damage)
		}
		rounds = append(rounds, swings)
	}
	return rounds
}

func meanOfInts(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	total := 0
	for _, x := range xs {
		total += x
	}
	return float64(total) / float64(len(xs))
}

func TestSurpriseRound_ExactlyOneSwingIsUpgraded(t *testing.T) {
	const (
		surpriseKnob = 8.0
		rounds       = 600
	)
	pinOpeningStrikeBalance(t, surpriseKnob)

	// ── Deterministic pins: the wiring, before any dice ──────────────────
	attacker := openingStrikeCombatant(t, "Ambusher")
	defender := openingStrikeCombatant(t, "Mark")

	plan := buildAttackPlan(attacker, defender)
	if plan.totalSwings < 3 {
		t.Fatalf("plan throws %d swings; this test cannot detect a round-scoped flag with fewer than 3",
			plan.totalSwings)
	}

	sdp := buildDamageParams(attacker, defender, plan.weapons[0], 0, User)
	// CritDamageMultiplier(20) = CritDamageBase 2.0 + CritDamagePerSkill 0.05 × 20.
	wantOpeningMult := (2.0 + 0.05*openingStrikeSkullduggery) * surpriseKnob
	if math.Abs(sdp.openingStrikeMult-wantOpeningMult) > 1e-9 {
		t.Fatalf("openingStrikeMult = %.4f, want %.4f (skullduggery crit worth × SurpriseOpeningStrikeMultiplier) — buildDamageParams is not wired",
			sdp.openingStrikeMult, wantOpeningMult)
	}

	critMean := sdp.rawDmgForCrit * sdp.critDmgMult
	stackedMean := critMean * sdp.openingStrikeMult
	// A stacked swing rolls around 16× an ordinary crit. The bar sits at 4×:
	// an ordinary crit would need roughly +20σ to reach it, and a stacked
	// strike roughly -5σ to fall under it. Neither happens in 600 rounds.
	stackedBar := 4.0 * critMean
	if stackedMean < 3.0*stackedBar {
		t.Fatalf("stacked mean %.1f is too close to the %.1f bar for a clean separation", stackedMean, stackedBar)
	}

	// ── The live rounds ──────────────────────────────────────────────────
	surpriseRounds := runOpeningStrikeRounds(t, rounds, true)
	plainRounds := runOpeningStrikeRounds(t, rounds, false)

	totalStacked := 0
	laterSurprise := []int{}
	for i, swings := range surpriseRounds {
		stacked := 0
		for _, dmg := range swings {
			if float64(dmg) >= stackedBar {
				stacked++
			}
		}
		if stacked > 1 {
			t.Fatalf("round %d threw %d swings and %d of them carried the opening-strike stack (damages %v, bar %.1f) — the flag is round-scoped, not per-swing: EVERY winning swing is being upgraded",
				i, len(swings), stacked, swings, stackedBar)
		}
		totalStacked += stacked
		laterSurprise = append(laterSurprise, swings[1:]...)
	}

	if totalStacked < rounds/5 {
		t.Fatalf("only %d of %d rounds landed an opening strike — the mechanism is not firing at all", totalStacked, rounds)
	}

	laterPlain := []int{}
	for _, swings := range plainRounds {
		laterPlain = append(laterPlain, swings[1:]...)
	}

	gotLater := meanOfInts(laterSurprise)
	wantLater := meanOfInts(laterPlain)
	t.Logf("opening strikes: %d/%d rounds; later-swing mean surprise %.2f vs plain %.2f (crit mean %.1f, stacked mean %.1f, bar %.1f)",
		totalStacked, rounds, gotLater, wantLater, critMean, stackedMean, stackedBar)

	if delta := math.Abs(gotLater-wantLater) / wantLater; delta > 0.15 {
		t.Fatalf("mean damage of NON-opening swings is %.2f in a surprise round vs %.2f in an ordinary one (%+.1f%%) — later swings must roll the ordinary mitigated mean, not the ambush's",
			gotLater, wantLater, delta*100)
	}
}
