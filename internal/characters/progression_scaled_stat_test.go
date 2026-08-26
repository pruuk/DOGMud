package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// pinCertainStatProgressionForTest makes a stat roll on a RANK 0 character
// succeed with certainty at bonusMultiplier 1.0.
//
// CalculateProgressionChance(0, softCap) is base*exp(0) == BaseProgressionChance,
// and ProgressionChanceForStat then multiplies by the bonus, the mutation
// multiplier (1.0 with no mutations), the per-stat multiplier (1.0 when
// StatProgressionMultipliers has no entry -- a test binary never loads
// config.yaml, so the shipped dex override is absent) and StatProgressionRate.
// Pinning base and rate to exactly 1.0 therefore makes chance == 1.0 at
// multiplier 1.0, and ProgressionRollThreshold(1.0) == progressionRollDenominator,
// which util.Rand can never equal or exceed. So a FRESH character progresses on
// its first call with probability 1, and any multiplier below 1.0 does not.
//
// This is what lets the delegation test below be exact rather than statistical:
// the only randomness left is on the deliberately-broken side.
//
// Only rank 0 is certain -- the curve decays from rank 1 onwards -- so callers
// must use a fresh character per call.
func pinCertainStatProgressionForTest(t *testing.T) {
	t.Helper()
	pinConfigForTest(t)
	cfg := configs.GetConfig()
	cfg.Balance.BaseProgressionChance = 1.0
	cfg.Balance.StatProgressionRate = 1.0
	cfg.Balance.StatProgressionMultipliers = nil
	configs.SetConfigForTest(t, cfg)
}

// OnStatUseScaled must actually honour its multiplier in BOTH directions.
//
// BOTH assertions are required and neither may be deleted. A multiplier of 0.0
// short-circuits inside CheckStatProgression (chance <= 0 returns false before
// any roll), so the "0.0 never advances" half alone would pass just as happily
// against a function that progresses nothing at all. The 1.0 half is what proves
// the path is live; the 0.0 half is what proves the multiplier is read.
//
// willpower, not dexterity: dexterity is the one stat with a shipped
// StatProgressionMultipliers override (0.5). A test binary never loads
// config.yaml so the override would not apply, but naming dexterity here invites
// a later reader to believe the test exercises it.
func TestOnStatUseScaled_HonoursItsMultiplier(t *testing.T) {
	pinConfigForTest(t)

	const trials = 600
	const stat = "willpower"

	// Multiplier 0.0: no advancement, ever.
	zero := newProgressionTestCharacter(t)
	before := zero.GetStatTraining(stat)
	for i := 0; i < trials; i++ {
		if zero.OnStatUseScaled(stat, 0, 0.0) {
			t.Fatalf("OnStatUseScaled at multiplier 0.0 reported a gain on trial %d", i)
		}
	}
	if after := zero.GetStatTraining(stat); after != before {
		t.Errorf("multiplier 0.0 advanced %s training from %d to %d over %d trials; a zero multiplier must never progress", stat, before, after, trials)
	}

	// Multiplier 1.0: advancement. At rank 0 the shipped-default test config is
	// roughly a 30% chance per use, so 600 trials failing to advance at all is
	// not a flake, it is a broken path.
	one := newProgressionTestCharacter(t)
	beforeOne := one.GetStatTraining(stat)
	for i := 0; i < trials; i++ {
		one.OnStatUseScaled(stat, 0, 1.0)
	}
	if after := one.GetStatTraining(stat); after <= beforeOne {
		t.Errorf("multiplier 1.0 left %s training at %d over %d trials; expected advancement", stat, after, trials)
	}
}

// OnStatUse must BE OnStatUseScaled at 1.0, not a parallel copy that can drift.
//
// Exact, not statistical: under pinCertainStatProgressionForTest a rank 0 roll at
// multiplier 1.0 succeeds with probability 1, so every fresh character must gain
// exactly one point on its first call. Any delegation at a multiplier below 1.0
// stops being certain, and the third assertion demonstrates that by showing 0.5
// genuinely differs under this same config -- i.e. the first two assertions have
// something real to catch.
func TestOnStatUse_DelegatesToOnStatUseScaledAtOne(t *testing.T) {
	pinCertainStatProgressionForTest(t)

	const stat = "willpower"
	const freshChars = 30

	for i := 0; i < freshChars; i++ {
		c := newProgressionTestCharacter(t)
		if !c.OnStatUse(stat, 0) {
			t.Fatalf("OnStatUse did not progress a rank 0 %s on character %d, but chance is pinned to 1.0", stat, i)
		}
		if got := c.GetStatTraining(stat); got != 1 {
			t.Fatalf("OnStatUse left %s training at %d on character %d, want 1", stat, got, i)
		}
		if got := c.GetStatUseCount(stat); got != 1 {
			t.Fatalf("OnStatUse recorded %d uses of %s on character %d, want 1", got, stat, i)
		}
	}

	for i := 0; i < freshChars; i++ {
		c := newProgressionTestCharacter(t)
		if !c.OnStatUseScaled(stat, 0, 1.0) {
			t.Fatalf("OnStatUseScaled at 1.0 did not progress a rank 0 %s on character %d, but chance is pinned to 1.0", stat, i)
		}
		if got := c.GetStatTraining(stat); got != 1 {
			t.Fatalf("OnStatUseScaled at 1.0 left %s training at %d on character %d, want 1", stat, got, i)
		}
	}

	// Proof the two loops above can fail: a multiplier of 0.5 under this same
	// pinned config is a coin flip, so across 30 fresh characters it cannot come
	// back certain (P(all 30 succeed) is about 1e-9). If this ever passes, the
	// multiplier is being ignored and the certainty assertions above are vacuous.
	allSucceeded := true
	for i := 0; i < freshChars; i++ {
		c := newProgressionTestCharacter(t)
		if !c.OnStatUseScaled(stat, 0, 0.5) {
			allSucceeded = false
			break
		}
	}
	if allSucceeded {
		t.Errorf("OnStatUseScaled at 0.5 progressed all %d fresh characters; the multiplier is not reaching the chance expression", freshChars)
	}
}
