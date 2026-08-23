package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
)

// OnDamageTaken is vitality's unobscured faucet.
//
// Vitality's other two paths are both RISK-GATED: regen only pays while a pool
// is depleted, and crit-toughen only fires when you are critted. Draining health
// means risking death; getting critted means fighting something that can kill
// you. The only SAFE route was the obscure trick of encumbering yourself to pin
// stamina near empty, which measured ~59x ordinary play. Absorbing a real hit is
// the plentiful event toughness should actually come from.
//
// These assert on StatUseCount, not on a rank change: a progression roll is
// probabilistic and a test that waits for one is flaky by construction.
// TrackStatUse fires exactly once per qualifying stat per call, so the counter
// is the deterministic observable.

func damageTestChar(t *testing.T, name string) *Character {
	t.Helper()
	c := New()
	c.Name = name
	c.StatUseCount = map[string]int{}
	return c
}

func TestOnDamageTaken_BelowThreshold_TrainsNothing(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Chipped")

	// 1% of the pool, well under the 5% gate. This is the AFK case: parking in a
	// zone that cannot hurt you must not farm progression off chip damage.
	c.OnDamageTaken(PoolHealth, c.HealthMax.Value/100, 0)

	if len(c.StatUseCount) != 0 {
		t.Errorf("chip damage trained %v; the threshold must gate it out", c.StatUseCount)
	}
}

func TestOnDamageTaken_AtThreshold_TrainsBothPoolStats(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Hurt")

	// A hit removing 10% of health, comfortably over the gate.
	c.OnDamageTaken(PoolHealth, c.HealthMax.Value/10, 0)

	if got := c.StatUseCount["vitality"]; got != 1 {
		t.Errorf("vitality trained %d times, want 1", got)
	}
	if got := c.StatUseCount["willpower"]; got != 1 {
		t.Errorf("willpower trained %d times, want 1", got)
	}
}

// Conviction damage trains willpower but explicitly NOT charisma.
//
// This is the taunt case. defy's award (AwardDefenceProgression) fires whenever
// the contest ran, win or lose, and trains rhetoric, whose primary stat is
// charisma. Including charisma here would award the same stat twice for one
// event -- a plain taunt moved the defender's charisma by 2 instead of 1, which
// taunt_collapse_test.go's drift guard correctly caught.
//
// It bites on conviction and nowhere else because conviction damage is ~3x
// larger relative to its pool, so a partially-mitigated taunt still clears the
// threshold where a partially-mitigated melee hit does not.
func TestOnDamageTaken_ConvictionDamage_TrainsWillpowerNotCharisma(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Taunted")

	c.OnDamageTaken(PoolConviction, c.ConvictionMax.Value/10, 0)

	if got := c.StatUseCount["willpower"]; got != 1 {
		t.Errorf("willpower trained %d times, want 1", got)
	}
	if got := c.StatUseCount["charisma"]; got != 0 {
		t.Errorf("conviction damage trained charisma %d times, want 0: defy's "+
			"own award already trains it via rhetoric", got)
	}
	if got := c.StatUseCount["vitality"]; got != 0 {
		t.Errorf("conviction damage trained vitality %d times, want 0", got)
	}
}

func TestOnDamageTaken_StaminaDamage_TrainsStrengthAndVitality(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Winded")

	c.OnDamageTaken(PoolStamina, c.StaminaMax.Value/10, 0)

	if got := c.StatUseCount["strength"]; got != 1 {
		t.Errorf("strength trained %d times, want 1", got)
	}
	if got := c.StatUseCount["vitality"]; got != 1 {
		t.Errorf("vitality trained %d times, want 1", got)
	}
}

// The gate is measured against the REACHABLE max, not the raw one. A character
// whose pool is mostly reserved cannot reach the raw ceiling, so holding them to
// a share of it would make the faucet progressively harder to trigger the more
// gear they wear -- the same class of mistake as the retired value floor, which
// let equipment make a stat harder to train.
func TestOnDamageTaken_ThresholdUsesReachableMax(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Reserved")

	max := c.ConvictionMax.Value
	reserveConvictionForTest(t, c, max/2)

	// 3% of the RAW max is under the 5% gate, but it is 6% of what this
	// character can actually reach, so it must qualify.
	hit := max * 3 / 100
	if hit < 1 {
		t.Skip("conviction pool too small for a meaningful 3% slice")
	}
	c.OnDamageTaken(PoolConviction, hit, 0)

	if got := c.StatUseCount["willpower"]; got != 1 {
		t.Errorf("willpower trained %d times, want 1: the gate must use the "+
			"reachable max, not the raw one", got)
	}
}

func TestOnDamageTaken_ZeroOrNegative_TrainsNothing(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Untouched")

	c.OnDamageTaken(PoolHealth, 0, 0)
	c.OnDamageTaken(PoolHealth, -50, 0)

	if len(c.StatUseCount) != 0 {
		t.Errorf("a non-positive hit trained %v; want nothing", c.StatUseCount)
	}
}

// ApplyHarm is the universal damage seam -- ApplyHealthChange delegates to it
// for negative deltas, so melee, spells, maneuvers, DoT and toxicity all pass
// through this one point. Pinning that the hook is actually wired there, not
// just that OnDamageTaken works when called directly.
func TestApplyHarm_FiresTheDamageTakenFaucet(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Struck")

	c.ApplyHarm(PoolHealth, c.HealthMax.Value/10, state.ActorRef{})

	if got := c.StatUseCount["vitality"]; got != 1 {
		t.Errorf("ApplyHarm trained vitality %d times, want 1 -- the hook in "+
			"ApplyHarm is what makes every damage source a faucet", got)
	}
}
