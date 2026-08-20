package combat

// U6b Task 10 — the counter tier. A defensive crit on a seam-resolved channel
// earns the defender one free answering swing (riposte's mechanism, priced by
// CounterDamagePercent), routed BACK through the channel seam with IsCounter
// so the original attacker defends it (and is charged and progressed for that
// defence — the countered-party economy) and so the tier can never recurse.

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
)

// pinCounterConfig pins the admission knobs plus the counter knob. Damage
// still rolls through dice.RollStat (the dice package caches its spread
// factor at init; configs cannot zero it), so damage assertions below use
// sampled means, never single-roll equality.
func pinCounterConfig(t *testing.T, counterPct float64) {
	t.Helper()
	pinDefenceAdmissionConfig(t)
	cfg := configs.GetConfig()
	cfg.Balance.CounterDamagePercent = configs.ConfigFloat(counterPct)
	configs.SetConfigForTest(t, cfg)
}

// counterTestPair returns a counterer (the one who earned the counter) and
// the countered original attacker, both stat-100 and deep-pooled.
func counterTestPair() (counterer, countered *characters.Character) {
	counterer = characters.New()
	counterer.Stats.Strength.Base = 100
	counterer.Stats.Strength.Recalculate()
	counterer.Health = 100000
	counterer.HealthMax.Value = 100000
	counterer.Stamina = 200
	counterer.StaminaMax.Value = 200

	countered = characters.New()
	countered.Stats.Strength.Base = 100
	countered.Stats.Strength.Recalculate()
	countered.Health = 100000
	countered.HealthMax.Value = 100000
	countered.Stamina = 200
	countered.StaminaMax.Value = 200
	return counterer, countered
}

// counterWinRunner: the counter-swing wins its contest cleanly (no crit, no
// fumble on either side).
func counterWinRunner(calls *int) func(float64, []contest.Entry) contest.Result {
	return func(atkScore float64, entries []contest.Entry) contest.Result {
		*calls++
		stdDev := dice.StdDevFor(atkScore)
		if stdDev <= 0 {
			stdDev = 15
		}
		margin := 0.5 * stdDev * math.Sqrt2
		return contest.Result{
			Contested: true,
			Success:   true,
			Winner:    entries[0].Name,
			Margin:    margin,
			AttackRoll: dice.RollResult{
				Mean: atkScore, StdDev: stdDev, ZScore: 0.5,
			},
			DefenseRoll: dice.RollResult{
				Mean: entries[0].Score, StdDev: stdDev, ZScore: -0.5,
			},
		}
	}
}

// counterDefensiveCritRunner: EVERY contest resolves as a decisive defensive
// win (a defensive crit). Used to prove the tier cannot chain off itself.
func counterDefensiveCritRunner(calls *int) func(float64, []contest.Entry) contest.Result {
	return func(atkScore float64, entries []contest.Entry) contest.Result {
		*calls++
		stdDev := dice.StdDevFor(atkScore)
		if stdDev <= 0 {
			stdDev = 15
		}
		return contest.Result{
			Contested: true,
			Success:   false,
			Winner:    entries[0].Name,
			Margin:    -2.5 * stdDev * math.Sqrt2,
			AttackRoll: dice.RollResult{
				Mean: atkScore, StdDev: stdDev, ZScore: -1.0,
			},
			DefenseRoll: dice.RollResult{
				Mean: entries[0].Score, StdDev: stdDev, ZScore: 2.5,
			},
		}
	}
}

// The reach gate: attacker and defender must share a room. The cross-room
// shot is the ONE uncounterable attack — a property of the weapon, decided by
// the owner, not a wiring hole.
func TestExecuteCounter_ReachGate(t *testing.T) {
	pinCounterConfig(t, 0.5)

	counterer, countered := counterTestPair()
	calls := 0
	restore := SetChannelAttackContestRunnerForTest(counterWinRunner(&calls))
	t.Cleanup(restore)

	// Cross-room: no counter, no contest, no damage.
	res := ExecuteCounter(counterer, countered, ChannelRanged, false)
	if res.Countered {
		t.Error("a cross-room shot was countered; it is the one uncounterable attack")
	}
	if calls != 0 {
		t.Errorf("cross-room counter ran %d contests, want 0", calls)
	}
	if countered.Health != 100000 {
		t.Error("cross-room counter dealt damage")
	}

	// Same room: the counter fires, contested through the seam, and the
	// winning swing lands damage on the original attacker.
	res = ExecuteCounter(counterer, countered, ChannelRanged, true)
	if !res.Countered {
		t.Fatal("same-room defensive crit did not counter")
	}
	if calls != 1 {
		t.Errorf("counter ran %d contests, want exactly 1 (the counter-swing's own)", calls)
	}
	if res.Damage <= 0 || countered.Health >= 100000 {
		t.Errorf("winning counter-swing dealt %d damage (health %d), want > 0",
			res.Damage, countered.Health)
	}
	if !res.Move.IsCounter {
		t.Error("the counter-swing's seam result must carry IsCounter")
	}
}

// A counter never earns a counter: the swing carries IsCounter through the
// seam, and even when the countered party crit-defends the counter-swing,
// exactly ONE contest runs — the tier never fires from its own result.
func TestExecuteCounter_NoRecursion(t *testing.T) {
	pinCounterConfig(t, 0.5)

	counterer, countered := counterTestPair()
	calls := 0
	restore := SetChannelAttackContestRunnerForTest(counterDefensiveCritRunner(&calls))
	t.Cleanup(restore)

	res := ExecuteCounter(counterer, countered, ChannelSpellMental, true)
	if !res.Countered {
		t.Fatal("counter did not fire")
	}
	if calls != 1 {
		t.Fatalf("counter chain ran %d contests, want exactly 1 — counters never recurse", calls)
	}
	if !res.Move.Defence.DefensiveCrit {
		t.Fatal("fixture error: the counter-swing was supposed to be crit-defended")
	}
	if !res.Move.IsCounter {
		t.Error("the counter-swing's seam result must carry IsCounter")
	}
	if res.Damage != 0 || countered.Health != 100000 {
		t.Errorf("crit-defended counter dealt %d damage, want 0", res.Damage)
	}
	if counterer.Health != 100000 {
		t.Error("a counter-swing that was crit-defended must not itself be countered")
	}
}

// The countered-party economy, named: a counter-swing routed through the seam
// means the ORIGINAL ATTACKER, now defending the counter, is CHARGED for that
// defence and PROGRESSED by it. "Counters are free" is true only for the
// counterer.
func TestExecuteCounter_CounteredPartyEconomy(t *testing.T) {
	pinCounterConfig(t, 0.5)

	counterer, countered := counterTestPair()
	calls := 0
	restore := SetChannelAttackContestRunnerForTest(counterWinRunner(&calls))
	t.Cleanup(restore)

	staminaBefore := countered.Stamina
	usesBefore := countered.GetSkillUseCount("unarmed-combat")

	res := ExecuteCounter(counterer, countered, ChannelSpellMental, true)
	if !res.Countered {
		t.Fatal("counter did not fire")
	}
	if res.Move.Defence.Cost.Status == characters.CostNoCharge {
		t.Error("the countered party's defence of the counter was not charged")
	}
	if countered.Stamina >= staminaBefore {
		t.Errorf("defending the counter cost nothing: stamina %d -> %d",
			staminaBefore, countered.Stamina)
	}
	// A bare-handed countered party defends with dodge, which trains
	// unarmed-combat: the counter TEACHES the countered party.
	if got := countered.GetSkillUseCount("unarmed-combat") - usesBefore; got != 1 {
		t.Errorf("defence progression fired %d events for the countered party, want 1", got)
	}
	// The counterer paid nothing: the counter is free for the counterer.
	if counterer.Stamina != 200 {
		t.Errorf("the counterer was charged %d stamina for a free counter",
			200-counterer.Stamina)
	}
}

// CounterDamagePercent prices the swing, and 0 is the documented off-switch:
// no fallback multiplier may sneak in through CalcRawDamage's itemMult<=0
// guard (which would silently restore 0.30 of weapon damage).
func TestExecuteCounter_KnobPricesTheSwingAndZeroDisables(t *testing.T) {
	calls := 0
	restore := SetChannelAttackContestRunnerForTest(counterWinRunner(&calls))
	t.Cleanup(restore)

	// dice.RollStat variance cannot be zeroed from configs, so compare
	// SAMPLED MEANS: at N=200 the standard error is ~1% of the mean, so a
	// 4x knob ratio outside (3, 5) is a real wiring failure, not noise.
	const samples = 200
	meanDamageAt := func(t *testing.T, pct float64) float64 {
		t.Helper()
		pinCounterConfig(t, pct)
		counterer, countered := counterTestPair()
		total := 0
		for i := 0; i < samples; i++ {
			countered.Health = 100000
			countered.Stamina = 200
			res := ExecuteCounter(counterer, countered, ChannelRanged, true)
			if !res.Countered || res.Damage <= 0 {
				t.Fatalf("fixture error: winning counter did not land (countered=%v dmg=%d)",
					res.Countered, res.Damage)
			}
			total += res.Damage
		}
		return float64(total) / samples
	}

	meanHalf := meanDamageAt(t, 0.5)
	meanDouble := meanDamageAt(t, 2.0)
	ratio := meanDouble / meanHalf
	if ratio < 3 || ratio > 5 {
		t.Errorf("knob does not price the swing: mean %.1f at 0.5 vs %.1f at 2.0 (ratio %.2f, want ~4)",
			meanHalf, meanDouble, ratio)
	}

	// Zero disables the tier outright — no contest, no damage.
	pinCounterConfig(t, 0)
	callsBefore := calls
	countererC, counteredC := counterTestPair()
	res := ExecuteCounter(countererC, counteredC, ChannelRanged, true)
	if res.Countered || calls != callsBefore || counteredC.Health != 100000 {
		t.Errorf("CounterDamagePercent 0 did not disable the tier: countered=%v contests=%d",
			res.Countered, calls-callsBefore)
	}
}
