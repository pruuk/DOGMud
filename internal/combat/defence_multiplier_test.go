package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/dice"
)

// U6 Task 10. A defensive win stopped being a clean miss: it now removes a
// fraction of the damage that scales with how decisively the defence won.
//
// The two endpoints are the whole contract. A bare win (margin ~0, the defence
// scraped it) removes half. A win AT the crit threshold removes all of it --
// which is what makes the curve continuous with the defensive crit sitting just
// past it, rather than a cliff where one extra thousandth of a standard
// deviation jumps mitigation from 50% to 100%.

const mitigEpsilon = 1e-9

func TestDefenceMitigation_BareWinRemovesHalf(t *testing.T) {
	if got := DefenceMitigation(0); math.Abs(got-0.5) > mitigEpsilon {
		t.Errorf("DefenceMitigation(0) = %v, want 0.5", got)
	}
}

func TestDefenceMitigation_AtCritThresholdRemovesAll(t *testing.T) {
	if got := DefenceMitigation(ContestCritThreshold); math.Abs(got-1.0) > mitigEpsilon {
		t.Errorf("DefenceMitigation(ContestCritThreshold) = %v, want 1.0", got)
	}
}

// Past the crit threshold the margin is a defensive crit's territory and is
// resolved before this function is reached. If it ever does arrive here it must
// clamp rather than run past 1.0 and produce NEGATIVE damage.
func TestDefenceMitigation_ClampsAboveCritThreshold(t *testing.T) {
	for _, m := range []float64{
		ContestCritThreshold * 2,
		ContestCritThreshold * 100,
		math.Inf(1),
	} {
		if got := DefenceMitigation(m); math.Abs(got-1.0) > mitigEpsilon {
			t.Errorf("DefenceMitigation(%v) = %v, want 1.0", m, got)
		}
	}
}

// A negative margin is not a defensive win at all -- the caller has no business
// being here. Clamping to the bare floor rather than extrapolating below it
// means a mis-signed margin cannot silently AMPLIFY damage past 100%.
func TestDefenceMitigation_NegativeMarginClampsToFloor(t *testing.T) {
	for _, m := range []float64{-0.001, -5, -math.MaxFloat64} {
		if got := DefenceMitigation(m); math.Abs(got-0.5) > mitigEpsilon {
			t.Errorf("DefenceMitigation(%v) = %v, want 0.5", m, got)
		}
	}
}

// Skill raises the margin, so this monotonicity IS the design claim: more skill
// must never buy less mitigation.
func TestDefenceMitigation_MonotonicAcrossTheCurve(t *testing.T) {
	const steps = 1000
	prev := math.Inf(-1)
	for i := 0; i <= steps; i++ {
		m := ContestCritThreshold * float64(i) / float64(steps)
		got := DefenceMitigation(m)
		if got < prev {
			t.Fatalf("DefenceMitigation decreased at margin %v: %v < %v", m, got, prev)
		}
		if got < 0.5 || got > 1.0 {
			t.Fatalf("DefenceMitigation(%v) = %v, outside [0.5, 1.0]", m, got)
		}
		prev = got
	}
}

// The midpoint is stated separately from monotonicity so a curve that swapped
// linear for, say, a quadratic would fail loudly instead of passing on
// endpoints plus monotonicity alone.
func TestDefenceMitigation_MidpointIsLinear(t *testing.T) {
	if got := DefenceMitigation(ContestCritThreshold / 2); math.Abs(got-0.75) > mitigEpsilon {
		t.Errorf("DefenceMitigation(half threshold) = %v, want 0.75", got)
	}
}

// ─── the curve where it is actually consumed ────────────────────────────────

// defenceWinBest builds a settled DEFENCE win with a real normaliser on the
// defence roll. mockBestDefense in hitroll_test.go leaves StdDev at zero, which
// makes normalizedDefenseMargin return ok == false -- fine for the crit tests it
// was written for, useless here, because every margin would collapse to the
// bare floor and the curve would never be exercised.
//
// margin is DEFENCE-POSITIVE (see the sign conversion in runBestOfAllDefense),
// so a POSITIVE margin here means the defence won.
func defenceWinBest(rawMargin, defStdDev float64) bestDefenseResult {
	return bestDefenseResult{
		margin:      rawMargin,
		defenseType: characters.DefenseDodge,
		hitRoll:     dice.RollResult{Value: 100, ZScore: 0},
		defRoll:     dice.RollResult{Value: 100 + rawMargin, ZScore: 0, StdDev: defStdDev},
	}
}

// The headline behaviour change of U6 Task 10: a defensive win is no longer a
// clean miss.
func TestResolveDefenseOutcome_DefenceWinLandsPartialDamage(t *testing.T) {
	src, tgt := defenceFixture(1000)

	// Roughly one standard deviation of defensive margin -> half way up the
	// curve -> 75% mitigated -> 25% damage through.
	stdDev := 15.0
	best := defenceWinBest(stdDev*math.Sqrt2, stdDev)

	res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, false)

	if !res.hit {
		t.Fatal("a defensive win must now land as a partially deflected hit, not a clean miss")
	}
	if res.crit || res.defenseCrit || res.fumble {
		t.Fatalf("expected a plain defensive win, got crit=%v defenseCrit=%v fumble=%v",
			res.crit, res.defenseCrit, res.fumble)
	}
	if math.Abs(res.damageMult-0.25) > 1e-6 {
		t.Errorf("damageMult = %v, want 0.25 (a one-sigma defensive margin mitigates 75%%)", res.damageMult)
	}
}

// Mitigation must rise with the margin all the way to the crit threshold, or
// skill investment in defence stops paying somewhere in the middle -- the exact
// complaint the arc exists to answer.
func TestResolveDefenseOutcome_MitigationRisesWithDefensiveMargin(t *testing.T) {
	src, tgt := defenceFixture(1000)
	stdDev := 15.0
	scale := stdDev * math.Sqrt2

	prev := math.Inf(1)
	for i := 0; i <= 20; i++ {
		z := ContestCritThreshold * float64(i) / 20
		// Nudge off an exact zero margin: margin <= 0 is an ATTACK win and would
		// take a different branch entirely.
		raw := z*scale + 1e-6

		res := resolveDefenseOutcomeCore(&AttackResult{}, defenceWinBest(raw, stdDev),
			src, tgt, 2.0, false, false, false)

		if res.damageMult > prev {
			t.Fatalf("damage got through MORE at defensive margin z=%.2f (%v) than at the "+
				"previous, smaller margin (%v)", z, res.damageMult, prev)
		}
		if res.damageMult < 0 || res.damageMult > 0.5 {
			t.Fatalf("damageMult %v at z=%.2f is outside the [0, 0.5] a defensive win allows",
				res.damageMult, z)
		}
		prev = res.damageMult
	}
}

// A FLOORED save carries the +-1 SENTINEL margin, which is in raw score units
// and is not a margin at all. This pins the special case in the defence-won
// branch. Its companion below is the evidence for why that special case exists
// rather than being redundant.
func TestResolveDefenseOutcome_FlooredSaveTakesTheBareFloor(t *testing.T) {
	src, tgt := defenceFixture(1000)

	for _, stdDev := range []float64{1.0, 15.0, 75.0} {
		best := defenceWinBest(1, stdDev) // margin +1 == the defence-side sentinel
		best.floored = true

		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, false)

		if !res.hit {
			t.Fatalf("stdDev %v: a floored save is still a defensive win and must land partial damage", stdDev)
		}
		if math.Abs(res.damageMult-0.5) > 1e-9 {
			t.Errorf("stdDev %v: floored save damageMult = %v, want exactly 0.5 (the BARE floor, "+
				"never a value derived from the sentinel)", stdDev, res.damageMult)
		}
	}
}

// This is the evidence that the special case above is load-bearing.
//
// The Task 10 brief expected the sentinel to normalise to a near-zero z and so
// yield 0.5 through the curve unaided. It does not: the sentinel is +-1 in RAW
// SCORE UNITS, so its normalised value is 1/(stdDev*sqrt2), which is small but
// never zero -- and StdDevFor clamps at 1.0 for means below 1, so a very weak
// defender's sentinel normalises to 0.71 z. Unaided, the curve would hand the
// WEAKEST defender the biggest floored save, which is backwards.
func TestFlooredSentinelDoesNotNormaliseToZero(t *testing.T) {
	for _, tc := range []struct {
		stdDev  float64
		minSkew float64 // how far the unaided curve strays from the bare 0.5
	}{
		{stdDev: 75.0, minSkew: 0.001}, // a strong defender: ~0.502
		{stdDev: 15.0, minSkew: 0.01},  // typical: ~0.512
		{stdDev: 1.0, minSkew: 0.15},   // StdDevFor's floor: ~0.677
	} {
		best := defenceWinBest(1, tc.stdDev)
		best.floored = true

		z, ok := normalizedDefenseMargin(best)
		if !ok {
			t.Fatalf("stdDev %v: normalizedDefenseMargin refused a sentinel margin; this test "+
				"is asserting about the wrong thing", tc.stdDev)
		}

		unaided := DefenceMitigation(z)
		if unaided-0.5 < tc.minSkew {
			t.Errorf("stdDev %v: sentinel normalised to z=%v giving mitigation %v; expected it to "+
				"stray at least %v above the bare 0.5, which is what makes the floored special "+
				"case necessary", tc.stdDev, z, unaided, tc.minSkew)
		}
		t.Logf("stdDev %5.1f: sentinel z=%.4f -> unaided mitigation %.4f (special-cased to 0.5000)",
			tc.stdDev, z, unaided)
	}
}

// The other return paths, pinned so a future edit cannot leave damageMult at its
// zero value and silently delete a swing's damage.
func TestResolveDefenseOutcome_DamageMultOnEveryPath(t *testing.T) {
	src, tgt := defenceFixture(1000)

	t.Run("attack won normally", func(t *testing.T) {
		best := defenceWinBest(-15*math.Sqrt2, 15) // negative margin == attack win
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, false)
		if !res.hit || res.damageMult != 1.0 {
			t.Errorf("hit=%v damageMult=%v, want true/1.0", res.hit, res.damageMult)
		}
	})

	t.Run("attack crit", func(t *testing.T) {
		best := defenceWinBest(-15*math.Sqrt2*3, 15)
		best.hitRoll.StdDev = 15
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, false)
		if !res.crit {
			t.Fatal("fixture did not produce an attack crit")
		}
		if res.damageMult != 1.0 {
			t.Errorf("an attack crit must deal full damage, got damageMult %v", res.damageMult)
		}
	})

	t.Run("defence crit", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2*3, 15)
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, false)
		if !res.defenseCrit {
			t.Fatal("fixture did not produce a defence crit")
		}
		if res.hit || res.damageMult != 0.0 {
			t.Errorf("a defence crit must fully negate, got hit=%v damageMult=%v", res.hit, res.damageMult)
		}
	})

	t.Run("attack fumble", func(t *testing.T) {
		best := defenceWinBest(-15*math.Sqrt2, 15)
		best.hitRoll.ZScore = -2.5
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, false)
		if !res.fumble || res.hit || res.damageMult != 0.0 {
			t.Errorf("fumble=%v hit=%v damageMult=%v, want true/false/0.0", res.fumble, res.hit, res.damageMult)
		}
	})

	t.Run("defence fumble", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2, 15)
		best.defRoll.ZScore = -2.5
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, false)
		if !res.hit || res.damageMult != 1.0 {
			t.Errorf("a defence fumble must concede full damage, got hit=%v damageMult=%v",
				res.hit, res.damageMult)
		}
	})
}

// M0b Task 1. A defence that HAPPENED must be narrated.
//
// RenderChannelDefenceMessages returned an empty triad whenever the message
// pool could not be resolved -- a missing pool, a missing intensity band, or
// audience lists of unequal length. Every caller then gates on
// `if triad.ToRoom == "" { return }`, so an empty room line discarded the
// attacker and defender lines along with it. The mechanics still resolved, so
// from the player's seat a spell simply stopped happening.
//
// The pool lookup is a raw string cast, items.DefenseType(out.DefenceType),
// which compiles for any string and yields nil for one with no authored pool.
// So a single rename is enough to silence a whole channel.
func TestRenderChannelDefenceMessages_UnknownPoolFallsBackToGenericText(t *testing.T) {
	out := ChannelDefenceResult{
		Defended:                true,
		DefenceType:             "no-such-defence-pool",
		NormalizedDefenceMargin: 0.6,
	}
	triad := RenderChannelDefenceMessages(out,
		ChannelDefenceIdentities{Attacker: "Grimwald", Defender: "Meirok"},
		"searing bolt")

	if triad.ToRoom == "" || triad.ToAttacker == "" || triad.ToDefender == "" {
		t.Fatalf("an unresolvable pool must fall back to generic text, got %+v", triad)
	}
	for name, msg := range map[string]string{
		"ToRoom":     string(triad.ToRoom),
		"ToAttacker": string(triad.ToAttacker),
		"ToDefender": string(triad.ToDefender),
	} {
		if !contains(msg, "searing bolt") {
			t.Errorf("%s should name the attack, got %q", name, msg)
		}
	}
	if !contains(string(triad.ToRoom), "Meirok") || !contains(string(triad.ToRoom), "Grimwald") {
		t.Errorf("the room line should name both actors, got %q", triad.ToRoom)
	}
}

// The direction a careless fix breaks. The attack WON, so there is no defence
// to narrate and the fallback must not invent one. This test passes BEFORE the
// fix, which is what makes it a guard rather than a restatement of it.
func TestRenderChannelDefenceMessages_UndefendedStaysEmpty(t *testing.T) {
	triad := RenderChannelDefenceMessages(
		ChannelDefenceResult{Defended: false},
		ChannelDefenceIdentities{Attacker: "Grimwald", Defender: "Meirok"},
		"searing bolt")
	if triad.ToRoom != "" || triad.ToAttacker != "" || triad.ToDefender != "" {
		t.Fatalf("an undefended attack must render nothing, got %+v", triad)
	}
}

// contains is a local helper so this file does not take a strings import for
// one predicate; the package already avoids it.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
