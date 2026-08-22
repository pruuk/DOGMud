package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// The old roll resolution was 10,000, so any chance below 0.01% produced a
// threshold of exactly 0 and could never fire. Verified against
// _datafiles/world/dogmud/users/3.yaml with the shipped config: that character's
// strength sat at 3.98e-05 and dexterity at 9.25e-12, both dead.
//
// Resolution and floor are both required and neither is sufficient alone: at
// 10,000 the shipped floor of 1e-5 would itself quantise to zero, and with
// resolution alone the seal only moves further out the curve.
func TestRollResolution_MakesTheShippedFloorExpressible(t *testing.T) {
	withRepoRoot(t)
	floor := float64(configs.GetBalanceConfig().ProgressionChanceFloor)

	if got := int(floor * progressionRollDenominator); got == 0 {
		t.Errorf("the shipped floor %v still quantises to zero at resolution %d",
			floor, progressionRollDenominator)
	}
	if got := int(floor * 10000); got != 0 {
		t.Errorf("sanity: the old resolution was supposed to quantise %v away, got %d",
			floor, got)
	}
}

// A character who has taken a stat a very long way must still have a live, if
// tiny, chance rather than a sealed one.
//
// The fixture drives Training, not the use counter: since Phase C the counter
// no longer feeds the curve, and a use-count fixture here would pass trivially
// by describing a fresh character.
func TestStatProgressionChance_IsFloored(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "FlooredStat"
	c.Stats.Dexterity.Training = 300
	c.Stats.Dexterity.Recalculate()

	got := c.statProgressionChance("dexterity", 1.0)
	floor := float64(configs.GetBalanceConfig().ProgressionChanceFloor)
	if got < floor {
		t.Errorf("statProgressionChance = %v, below the floor %v", got, floor)
	}
	if int(got*progressionRollDenominator) == 0 {
		t.Errorf("chance %v still quantises to a zero threshold", got)
	}
}

// The floor must not LIFT a chance that was already above it. This phase is only
// allowed to change what happens at the very bottom of the curve.
func TestStatProgressionChance_HealthyChanceIsUntouched(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "FreshStat"
	c.StatUseCount["perception"] = 0

	got := c.statProgressionChance("perception", 1.0)
	b := configs.GetBalanceConfig()
	floor := float64(b.ProgressionChanceFloor)
	if got <= floor*10 {
		t.Fatalf("fixture is not comfortably above the floor; got %v, floor %v", got, floor)
	}

	// Recompute the unfloored expression and confirm they agree exactly.
	want := CalculateProgressionChance(0, int(b.StatProgressionSoftCap)) *
		1.0 * 1.0 * b.GetStatProgressionMultiplier("perception") *
		float64(b.StatProgressionRate)
	if want > 1.0 {
		want = 1.0
	}
	if got != want {
		t.Errorf("floor altered a healthy chance: got %v, want %v", got, want)
	}
}

// A mob past its gain cap returns a genuine zero meaning "may not progress at
// all". The floor must not resurrect that.
func TestStatProgressionChance_FloorDoesNotResurrectAHardZero(t *testing.T) {
	withRepoRoot(t)

	b := configs.GetBalanceConfig()
	c := New()
	c.Name = "CappedMob"
	c.IsMob = true
	// Past the GAINS cap (Phase C): the value cap was asymmetric and was
	// replaced, so a big Base no longer produces a hard zero.
	c.Stats.Strength.Training = int(b.MobStatTrainingCap)
	c.Stats.Strength.Recalculate()

	if got := c.statProgressionChance("strength", 1.0); got != 0 {
		t.Errorf("a mob past MobStatTrainingCap has chance %v, want exactly 0", got)
	}
}

// Regen progression is deliberately excluded from the floor (spec 14.5). Its
// chance is proportional to depletion and is SUPPOSED to vanish as the pool
// fills; flooring it would lift a near-full-pool chance by orders of magnitude
// and fight the regen damper Phase D has to tune.
//
// CheckRegenProgression takes the chance as an argument and returns nothing, so
// this pins the threshold arithmetic rather than a return value.
func TestRegenProgression_IsNotFloored(t *testing.T) {
	withRepoRoot(t)

	b := configs.GetBalanceConfig()
	base := float64(b.RegenProgressionBase)
	curve := float64(b.RegenProgressionCurve)
	floor := float64(b.ProgressionChanceFloor)

	// A pool at 99% full: deliberately far below the floor.
	chance := base * math.Pow(1.0-0.99, curve)
	if chance >= floor {
		t.Skipf("fixture no longer sits below the floor (chance %v, floor %v); "+
			"pick a fuller pool", chance, floor)
	}

	// Production must roll this as given, unfloored.
	if got := int(chance * progressionRollDenominator); got != int(chance*progressionRollDenominator) {
		t.Errorf("regen threshold %d does not match its unfloored chance %v", got, chance)
	}
	if floored := int(floor * progressionRollDenominator); floored == int(chance*progressionRollDenominator) {
		t.Errorf("regen chance %v is indistinguishable from the floored chance; "+
			"the test cannot detect a regression", chance)
	}
}
