package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/state"
)

// OnDamageTaken is vitality's unobscured faucet, and trains ONLY vitality.
//
// Vitality's other two paths are both RISK-GATED: regen only pays while a pool is
// depleted, and crit-toughen only fires when you are critted. Draining health
// means risking death; getting critted means fighting something that can kill
// you. The only SAFE route was the obscure trick of encumbering yourself to pin
// stamina near empty, which measured ~59x ordinary play. Absorbing a real hit is
// the plentiful event toughness should actually come from.
//
// Scope matters here. An earlier draft mirrored the regen mapping and gave every
// pool two stats, which pulled in willpower and charisma and collided with the
// paths that already train them -- defy trains rhetoric (primary stat charisma)
// on every resolved taunt, and quell plus the magical toughen path both train
// willpower. Vitality collides with nothing, because no defence award trains it.
// That is what makes this faucet safe to hook at a seam which does not know the
// damage channel.
//
// These assert on StatUseCount, not on a rank change: a progression roll is
// probabilistic and a test that waits for one is flaky by construction.
// TrackStatUse fires exactly once per qualifying stat per call, so the counter is
// the deterministic observable.

// getBalanceThresholdForTest reads the live gate rather than hardcoding 0.05, so
// this test pins the BOUNDARY behaviour and not a particular tuning.
func getBalanceThresholdForTest() float64 {
	return float64(configs.GetBalanceConfig().DamageProgressionThresholdPct)
}

func damageTestChar(t *testing.T, name string) *Character {
	t.Helper()
	c := New()
	c.Name = name
	c.StatUseCount = map[string]int{}
	return c
}

func TestOnDamageTaken_HealthHit_TrainsVitalityOnly(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Hurt")

	// A hit removing 10% of health, comfortably over the gate.
	c.OnDamageTaken(PoolHealth, c.HealthMax.Value/10, 0)

	if got := c.StatUseCount["vitality"]; got != 1 {
		t.Errorf("vitality trained %d times, want 1", got)
	}
	if len(c.StatUseCount) != 1 {
		t.Errorf("health damage trained %v; it must train vitality and nothing "+
			"else -- willpower and charisma have their own paths and would be "+
			"double-awarded", c.StatUseCount)
	}
}

func TestOnDamageTaken_StaminaHit_TrainsVitality(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Winded")

	c.OnDamageTaken(PoolStamina, c.StaminaMax.Value/10, 0)

	if got := c.StatUseCount["vitality"]; got != 1 {
		t.Errorf("vitality trained %d times, want 1", got)
	}
	if len(c.StatUseCount) != 1 {
		t.Errorf("stamina damage trained %v; want vitality alone", c.StatUseCount)
	}
}

// Conviction is deliberately NOT a faucet. It is not a body pool, and wiring it
// up would be a charisma/willpower change with its own multipliers to re-solve.
// Concretely: defy's award fires on every resolved taunt, win or lose, and trains
// rhetoric, whose primary stat is charisma -- so a conviction faucet
// double-awarded charisma and tripped taunt_collapse_test.go's drift guard.
func TestOnDamageTaken_ConvictionDamage_TrainsNothing(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Taunted")

	c.OnDamageTaken(PoolConviction, c.ConvictionMax.Value/2, 0)

	if len(c.StatUseCount) != 0 {
		t.Errorf("conviction damage trained %v; it must train nothing -- see the "+
			"PoolProgressionStats doc comment", c.StatUseCount)
	}
}

// The threshold boundary. This is the AFK guard: without it, parking in a zone
// that cannot hurt you and walking away from the keyboard would farm vitality off
// chip damage overnight. Pinned either side of the 5% gate rather than at 1% vs
// 10%, so a change to the default is caught rather than absorbed.
func TestOnDamageTaken_ThresholdBoundary(t *testing.T) {
	withRepoRoot(t)

	pct := float64(getBalanceThresholdForTest())
	if pct <= 0 {
		t.Fatalf("DamageProgressionThresholdPct is %v, want > 0", pct)
	}

	// Each character's gate must be computed from ITS OWN pool: New() rolls
	// stats, so two characters do not share a HealthMax and sizing the second
	// hit from the first one's pool makes this pass or fail on the roll.
	below := damageTestChar(t, "JustUnder")
	belowMax := below.HealthMax.Value
	belowGate := pct * float64(belowMax)
	hit := int(belowGate) - 1
	if hit < 1 {
		t.Skip("health pool too small to straddle the threshold")
	}
	below.OnDamageTaken(PoolHealth, hit, 0)
	if len(below.StatUseCount) != 0 {
		t.Errorf("a hit of %d against max %d (gate %.2f) trained %v; it is under "+
			"the threshold and must train nothing",
			hit, belowMax, belowGate, below.StatUseCount)
	}

	above := damageTestChar(t, "JustOver")
	aboveMax := above.HealthMax.Value
	aboveGate := pct * float64(aboveMax)
	hit = int(aboveGate) + 1
	above.OnDamageTaken(PoolHealth, hit, 0)
	if got := above.StatUseCount["vitality"]; got != 1 {
		t.Errorf("a hit of %d against max %d (gate %.2f) trained vitality %d "+
			"times, want 1", hit, aboveMax, aboveGate, got)
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

// ApplyHarm is the universal damage seam -- ApplyHealthChange delegates to it for
// negative deltas, so melee, spells, maneuvers, damage-over-time and toxicity all
// pass through this one point. Pins that the hook is actually wired there, not
// merely that OnDamageTaken works when called directly.
func TestApplyHarm_FiresTheDamageTakenFaucet(t *testing.T) {
	withRepoRoot(t)
	c := damageTestChar(t, "Struck")

	c.ApplyHarm(PoolHealth, c.HealthMax.Value/10, state.ActorRef{})

	if got := c.StatUseCount["vitality"]; got != 1 {
		t.Errorf("ApplyHarm trained vitality %d times, want 1 -- the hook in "+
			"ApplyHarm is what makes every damage source a faucet", got)
	}
}
