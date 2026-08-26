package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// pinSituationalKnobs pins the two knobs the modifier table reads so the
// expected values below cannot drift with config defaults.
func pinSituationalKnobs(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.ProneAttackMultiplier = 0.80
	cfg.Balance.StaminaPenaltyMax = 0.28
	cfg.Balance.ResourcePenaltyCurve = 2.0
	configs.SetConfigForTest(t, cfg)
}

// newSituationalAttacker builds a standing attacker at full stamina.
func newSituationalAttacker(t *testing.T) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Stamina = 100
	c.StaminaMax.Value = 100
	return c
}

func forceProne(t *testing.T, c *characters.Character) {
	t.Helper()
	if c.Position == nil {
		c.Position = position.NewMachine()
	}
	r := state.TransitionReason{Trigger: "test_setup"}
	c.Position.ForceStanding(r)
	if err := c.Position.TransitionToProne(position.ProneData{}, r); err != nil {
		t.Fatalf("could not force prone: %v", err)
	}
}

// The DECLARED table: a healthy standing attacker takes no situational
// penalty on any channel.
func TestSituationalAttackMult_HealthyStandingIsUnity(t *testing.T) {
	pinSituationalKnobs(t)
	atk := newSituationalAttacker(t)
	for _, ch := range []AttackChannel{ChannelMelee, ChannelRanged, ChannelSpellPhysical, ChannelSpellMental, ChannelSocial} {
		if got := SituationalAttackMult(atk, ch); got != 1.0 {
			t.Errorf("channel %s: healthy standing attacker mult = %v, want 1.0", ch, got)
		}
	}
}

// Prone attacker: penalised on melee and ranged (the physical channels the
// specials also ride), NOT on spell or social — you cast/talk fine from the
// ground.
func TestSituationalAttackMult_ProneAttackerTable(t *testing.T) {
	pinSituationalKnobs(t)
	atk := newSituationalAttacker(t)
	forceProne(t, atk)

	for _, tc := range []struct {
		channel AttackChannel
		want    float64
	}{
		{ChannelMelee, 0.80},
		{ChannelRanged, 0.80},
		{ChannelSpellPhysical, 1.0},
		{ChannelSpellMental, 1.0},
		{ChannelSocial, 1.0},
	} {
		if got := SituationalAttackMult(atk, tc.channel); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("channel %s: prone attacker mult = %v, want %v", tc.channel, got, tc.want)
		}
	}
}

// Resource depletion reaches ACCURACY on the physical channels only. Spell
// and social already pay their depletion in the damage term (conviction), so
// the shared layer must NOT tax them a second time here.
func TestSituationalAttackMult_StaminaDepletionTable(t *testing.T) {
	pinSituationalKnobs(t)
	atk := newSituationalAttacker(t)
	atk.Stamina = 25 // 25% stamina

	want := ResourceMultiplier(atk.Stamina, atk.EffectivePoolMax(characters.PoolStamina),
		float64(configs.GetBalanceConfig().StaminaPenaltyMax))
	if want >= 1.0 {
		t.Fatalf("fixture is broken: depleted stamina produced no penalty (%v)", want)
	}

	for _, tc := range []struct {
		channel AttackChannel
		want    float64
	}{
		{ChannelMelee, want},
		{ChannelRanged, want},
		{ChannelSpellPhysical, 1.0},
		{ChannelSpellMental, 1.0},
		{ChannelSocial, 1.0},
	} {
		if got := SituationalAttackMult(atk, tc.channel); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("channel %s: depleted-stamina mult = %v, want %v", tc.channel, got, tc.want)
		}
	}
}

// Prone and depletion COMPOUND on the physical channels, matching melee's
// calcAttackScore, which this layer mirrors.
func TestSituationalAttackMult_ProneAndDepletionCompound(t *testing.T) {
	pinSituationalKnobs(t)
	atk := newSituationalAttacker(t)
	atk.Stamina = 25
	forceProne(t, atk)

	depletion := ResourceMultiplier(atk.Stamina, atk.EffectivePoolMax(characters.PoolStamina),
		float64(configs.GetBalanceConfig().StaminaPenaltyMax))
	want := 0.80 * depletion
	if got := SituationalAttackMult(atk, ChannelMelee); math.Abs(got-want) > 1e-9 {
		t.Errorf("melee prone+depleted mult = %v, want %v", got, want)
	}
}

func TestSituationalAttackMult_NilAttackerIsUnity(t *testing.T) {
	if got := SituationalAttackMult(nil, ChannelMelee); got != 1.0 {
		t.Errorf("nil attacker mult = %v, want 1.0", got)
	}
}

// ─── Sleeping forced crit ────────────────────────────────────────────────────

// publishEmptySnapshotAfter restores the package-level sleeping snapshot so a
// test cannot leak forced crits into its neighbours.
func publishEmptySnapshotAfter(t *testing.T) {
	t.Helper()
	prev := sleepingSnapshot
	t.Cleanup(func() { sleepingSnapshot = prev })
}

// A defender sleeping RIGHT NOW forces the crit, snapshot or no snapshot —
// the command-driven channels (cast/shoot/taunt/bash) resolve between round
// passes and must not need DoCombat's maps to honour the contract.
func TestSleepingForceCrit_LiveFlag(t *testing.T) {
	publishEmptySnapshotAfter(t)
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		15: {BuffId: 15, Name: "Sleeping", TriggerCount: 1000000000,
			Flags: []buffs.Flag{buffs.Sleeping}},
	}))
	def := characters.New()
	if SleepingForceCrit(def) {
		t.Fatal("an awake defender must not force a crit")
	}
	def.Buffs.List = append(def.Buffs.List, &buffs.Buff{BuffId: 15, TriggersLeft: 1000000000})
	def.Buffs.Validate(true)
	if !def.HasBuffFlag(buffs.Sleeping) {
		t.Fatal("fixture is broken: the Sleeping flag did not apply")
	}
	if !SleepingForceCrit(def) {
		t.Error("a defender with the live Sleeping flag must force the crit")
	}
}

// A defender woken mid-round (flag gone) still eats forced crits for the rest
// of the round via the round-start snapshot — the same window melee's
// snapshotSleepingVictims maps have always provided.
func TestSleepingForceCrit_SnapshotSurvivesWake(t *testing.T) {
	publishEmptySnapshotAfter(t)
	def := characters.New()
	def.MobInstanceId = 777

	round := util.GetRoundCount()
	PublishSleepingSnapshot(round, map[int]bool{}, map[int]bool{777: true})
	if !SleepingForceCrit(def) {
		t.Error("a round-start-sleeping defender must force the crit after waking")
	}

	// A stale snapshot (any other round) says nothing about THIS round.
	PublishSleepingSnapshot(round+1, map[int]bool{}, map[int]bool{777: true})
	if SleepingForceCrit(def) {
		t.Error("a snapshot for a different round must not force a crit")
	}
}

// ─── ForceCrit through the channel seam ──────────────────────────────────────

// decisiveDefenceWinRunner returns a contest where the defence won by a
// crit-worthy margin on a fumbled attack roll — the strongest possible
// defensive outcome, which ForceCrit must override completely.
func decisiveDefenceWinRunner(atkScore float64, entries []contest.Entry) contest.Result {
	stdDev := dice.StdDevFor(atkScore)
	if stdDev <= 0 {
		stdDev = 15
	}
	margin := -3.0 * stdDev * math.Sqrt2 // decisively defence-positive (attack-negative)
	return contest.Result{
		Contested: true,
		Success:   false,
		Winner:    entries[0].Name,
		Margin:    margin,
		AttackRoll: dice.RollResult{
			Value: atkScore - 2.5*stdDev, Mean: atkScore,
			StdDev: stdDev, ZScore: -2.5, // a fumble-grade roll
		},
		DefenseRoll: dice.RollResult{
			Value: atkScore - 2.5*stdDev - margin, Mean: entries[0].Score,
			StdDev: stdDev, ZScore: 3.0,
		},
	}
}

// ForceCrit mirrors melee's resolveDefenseOutcomeInner semantics exactly: the
// attack crits, cannot fumble, wins even when the defence took the margin,
// and lands at the full damage multiplier. The defence is still mounted —
// charged and identified — it just cannot change the outcome.
func TestResolveChannelAttack_ForceCritOverridesEverything(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	atk, def := newDefenceTestCharacter(t), newDefenceTestCharacter(t)

	s := side(148, 52)
	s.ForceCrit = true
	out := resolveChannelAttackWithRunner(ChannelSpellMental, s, atk, def, decisiveDefenceWinRunner)

	if !out.AttackerCrit {
		t.Error("ForceCrit did not set AttackerCrit")
	}
	if out.AttackerFumble {
		t.Error("a forced crit must suppress the fumble verdict (melee parity)")
	}
	if out.Defended {
		t.Error("a forced crit must force the WIN too — the defence cannot keep the margin")
	}
	if out.DefensiveCrit {
		t.Error("a forced crit must not report a defensive crit")
	}
	if out.DamageMultiplier != 1.0 {
		t.Errorf("forced crit damage multiplier = %v, want 1.0", out.DamageMultiplier)
	}
	if out.DefenceType == "" {
		t.Error("the defence was still mounted; its identity must survive for narration/cost")
	}
}

// Without ForceCrit the same contest is a decisive defensive win — the guard
// that the override above is doing the work, not the runner.
func TestResolveChannelAttack_SameContestWithoutForceCritIsDefended(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	atk, def := newDefenceTestCharacter(t), newDefenceTestCharacter(t)

	out := resolveChannelAttackWithRunner(ChannelSpellMental, side(148, 52), atk, def, decisiveDefenceWinRunner)
	if !out.Defended {
		t.Fatal("fixture is broken: the decisive defence win did not defend")
	}
	if out.AttackerCrit {
		t.Error("fixture is broken: the losing attack critted")
	}
}

// A forced crit against a defender with NO defence set still reports the crit
// (the uncontested early return must honour it too).
func TestResolveChannelAttack_ForceCritUncontested(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	atk := newDefenceTestCharacter(t)
	def := newDefenceTestCharacter(t)
	// Drive the not-contested early return via a runner that reports no
	// contest — the same shape RunContest returns for an empty entry set.
	s := side(148, 52)
	s.ForceCrit = true
	runner := func(_ float64, _ []contest.Entry) contest.Result {
		return contest.Result{Contested: false}
	}
	out := resolveChannelAttackWithRunner(ChannelSpellMental, s, atk, def, runner)
	if !out.AttackerCrit {
		t.Error("ForceCrit must survive an uncontested outcome")
	}
	if out.DamageMultiplier != 1.0 {
		t.Errorf("uncontested forced crit damage multiplier = %v, want 1.0", out.DamageMultiplier)
	}
}
