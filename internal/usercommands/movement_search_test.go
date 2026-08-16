package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
)

// setMovementSearchTrainChance pins the U7 Task 10 knob for the calling test.
// configs.SetConfigForTest assigns WITHOUT running Balance.Validate, which is
// what lets a deliberate zero survive -- validateMisc would otherwise replace an
// absent (zero) value with the 0.005 default. It self-registers the restore.
func setMovementSearchTrainChance(t *testing.T, chance float64) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.MovementSearchTrainChance = configs.ConfigFloat(chance)
	configs.SetConfigForTest(t, c)
}

// observeGateRate runs the gate n times and returns the observed fire rate.
func observeGateRate(n int) float64 {
	fired := 0
	for i := 0; i < n; i++ {
		if movementTrainsSearch() {
			fired++
		}
	}
	return float64(fired) / float64(n)
}

// TestMovementTrainsSearch_FiresAtConfiguredRate checks the shipped 1-in-200
// gate lands in the right neighbourhood.
//
// The accepted band (0.002 to 0.012 around an expected 0.005) is deliberately
// far wider than the sampling error would justify. At 100k trials the standard
// deviation of the observed rate is only about 0.0002, so a tight band would
// pass on statistics and fail on nothing but bad luck or a future change to the
// shared RNG seeding. The band is sized to catch the failures that matter --
// the gate wired to the wrong knob, inverted, off by an order of magnitude, or
// firing every move -- not to measure the RNG.
func TestMovementTrainsSearch_FiresAtConfiguredRate(t *testing.T) {
	setMovementSearchTrainChance(t, 0.005)

	rate := observeGateRate(100000)
	t.Logf("observed gate rate %v over 100000 trials (expected ~0.005)", rate)

	assert.Greater(t, rate, 0.002, "gate fires far too rarely for a 0.005 chance (observed %v)", rate)
	assert.Less(t, rate, 0.012, "gate fires far too often for a 0.005 chance (observed %v)", rate)
}

// TestMovementTrainsSearch_ZeroKnobDisables proves the feature can be switched
// off outright: at zero the gate must NEVER fire, not merely fire seldom.
func TestMovementTrainsSearch_ZeroKnobDisables(t *testing.T) {
	setMovementSearchTrainChance(t, 0)

	for i := 0; i < 200000; i++ {
		if movementTrainsSearch() {
			t.Fatalf("gate fired on iteration %d with MovementSearchTrainChance at zero", i)
		}
	}
}

// TestMovementTrainsSearch_NegativeKnobDisables covers the other "off" spelling.
// validateMisc leaves a negative value alone precisely so an operator can
// disable travel-training in config.yaml without the absent-means-default rule
// putting it back.
func TestMovementTrainsSearch_NegativeKnobDisables(t *testing.T) {
	setMovementSearchTrainChance(t, -1)

	for i := 0; i < 10000; i++ {
		if movementTrainsSearch() {
			t.Fatalf("gate fired on iteration %d with a negative MovementSearchTrainChance", i)
		}
	}
}

// TestMovementTrainsSearch_KnobIsRead is the test that would catch the rate
// being hardcoded: turn the knob up by two orders of magnitude and the observed
// rate must follow it, not stay near 0.005.
func TestMovementTrainsSearch_KnobIsRead(t *testing.T) {
	setMovementSearchTrainChance(t, 0.5)

	rate := observeGateRate(20000)
	t.Logf("observed gate rate %v over 20000 trials (expected ~0.5)", rate)

	assert.Greater(t, rate, 0.45, "gate ignored a 0.5 knob (observed %v)", rate)
	assert.Less(t, rate, 0.55, "gate ignored a 0.5 knob (observed %v)", rate)
}
