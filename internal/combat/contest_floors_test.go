package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// Tests for the floor-pair wrappers (roadmap U3 task 1).
//
// EVERY floor knob these wrappers read defaults to ZERO in a test binary. A Go
// test binary never loads _datafiles/config.yaml, and validateMisc only
// substitutes a default when the value is < 0 || > 0.50 -- 0 is in range and
// survives. So with no injection the two wrappers are behaviourally IDENTICAL
// and any "the channels differ" assertion passes vacuously. Every test below
// therefore pins the knobs it depends on explicitly, in both directions.
//
// Injection goes through configs.SetConfigForTest, which snapshots and restores
// via t.Cleanup itself. configs.GetBalanceConfig() returns Balance BY VALUE, so
// mutating what it returns is a no-op; and configs.AddOverlayOverrides has no
// reset API and would leak a pinned floor into grapple_test.go's statistical
// tests in this same package. Both wrappers re-read config on every call, so one
// SetConfigForTest per test function covers all of that function's calls.

// pinFloors installs a config with all four contest-floor knobs set to the
// given values, for the duration of the calling test.
//
// Callers must NOT use t.Parallel(): this replaces the package-global config,
// so two parallel tests would fight over it and each would see the other's
// floors.
func pinFloors(t *testing.T, maneuverHit, maneuverResist, spellHit, spellResist float64) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.MinManeuverHitChance = configs.ConfigFloat(maneuverHit)
	c.Balance.MinManeuverResistChance = configs.ConfigFloat(maneuverResist)
	c.Balance.MinSpellHitChance = configs.ConfigFloat(spellHit)
	c.Balance.MinSpellResistChance = configs.ConfigFloat(spellResist)
	configs.SetConfigForTest(t, c)
}

func TestRunWithManeuverFloors_DelegatesToCore(t *testing.T) {
	// Both maneuver floors pinned to 0 on purpose. config.yaml ships
	// MinManeuverResistChance: 0.05. This package's TestMain (hitroll_test.go)
	// does not load config today, so the knob happens to be 0 anyway -- but if
	// it ever starts loading config, an unpinned version of this test would flip
	// to Success=false carrying the -1 sentinel margin about one run in twenty.
	// Pinning makes the test independent of that decision.
	pinFloors(t, 0, 0, 0, 0)

	// Attacker 0.5 against defender -10000. The attack score is deliberately
	// BELOW dice.StdDevFor's `mean < 1.0` floor, so stdDev pins to 1.0 instead
	// of scaling with the score: the margin is Normal(10000.5, 1.41), thousands
	// of sigma clear of zero.
	//
	// Merely raising the attack score would NOT achieve that. contest.Run rolls
	// BOTH sides at StdDevFor(atkScore), so the spread grows with the score and
	// the margin's z converges to 1/(0.15*sqrt(2)) = 4.71 however large it gets
	// -- a ~1.2e-6 flake, not a certainty. Dropping under the floor is what
	// decouples spread from score.
	res := RunWithManeuverFloors(0.5, -10000)

	if !res.Contested {
		t.Errorf("Contested = false, want true (one entry was supplied)")
	}
	if !res.Success {
		t.Errorf("Success = false, want true (attacker 0.5 vs -10000, floors pinned to 0)")
	}
	if res.Margin <= 0 {
		t.Errorf("Margin = %v, want > 0 -- the core is ATTACK-POSITIVE", res.Margin)
	}
	if res.Winner != "" {
		t.Errorf("Winner = %q, want \"\" -- the single entry is deliberately unnamed", res.Winner)
	}
	if res.AttackRoll.StdDev <= 0 {
		t.Errorf("AttackRoll.StdDev = %v, want > 0 -- crit normalisation divides by it",
			res.AttackRoll.StdDev)
	}
}

func TestRunWithSpellFloors_DelegatesToCore(t *testing.T) {
	// Mirror of the maneuver case; the spell pair is pinned to 0 for the same
	// reason (config.yaml ships MinSpellResistChance: 0.05), and the scores are
	// chosen the same way -- see the comment there for why the attack score is
	// below dice.StdDevFor's `mean < 1.0` floor rather than merely large.
	pinFloors(t, 0, 0, 0, 0)

	res := RunWithSpellFloors(0.5, -10000)

	if !res.Contested {
		t.Errorf("Contested = false, want true (one entry was supplied)")
	}
	if !res.Success {
		t.Errorf("Success = false, want true (attacker 0.5 vs -10000, floors pinned to 0)")
	}
	if res.Margin <= 0 {
		t.Errorf("Margin = %v, want > 0 -- the core is ATTACK-POSITIVE", res.Margin)
	}
	if res.Winner != "" {
		t.Errorf("Winner = %q, want \"\" -- the single entry is deliberately unnamed", res.Winner)
	}
	if res.AttackRoll.StdDev <= 0 {
		t.Errorf("AttackRoll.StdDev = %v, want > 0 -- crit normalisation divides by it",
			res.AttackRoll.StdDev)
	}
}

// TestWrappers_ReadTheirOwnFloorPair is the guard against the copy-paste that
// points BOTH wrappers at ManeuverFloors(). That mistake compiles, passes both
// delegation tests above, and silently retunes the spell channel.
//
// It works by driving the two pairs apart: the maneuver hit floor is pinned high
// while the spell hit floor is pinned to zero, and a hopeless attacker is run
// through each. Only the maneuver wrapper may ever report Floored.
func TestWrappers_ReadTheirOwnFloorPair(t *testing.T) {
	// Resist floors stay 0 so the hopeless attacker can only ever be flipped by
	// a HIT floor -- the one knob that differs between the two pairs here.
	pinFloors(t, 0.5, 0, 0, 0)

	const runs = 300

	maneuverFloored := 0
	spellFloored := 0
	for i := 0; i < runs; i++ {
		// Score 1 against 10000: the attacker loses every contest on the roll,
		// so every flip observed came from the hit floor.
		if RunWithManeuverFloors(1, 10000).Floored {
			maneuverFloored++
		}
		if RunWithSpellFloors(1, 10000).Floored {
			spellFloored++
		}
	}

	// p = 0.5 over 300 trials centres on 150; 50 is ~11 sigma below that.
	if maneuverFloored < 50 {
		t.Errorf("RunWithManeuverFloors floored %d/%d times, want a substantial share "+
			"(MinManeuverHitChance pinned to 0.5) -- the maneuver wrapper is not reading the maneuver pair",
			maneuverFloored, runs)
	}
	if spellFloored != 0 {
		t.Errorf("RunWithSpellFloors floored %d/%d times, want 0 (both spell floors pinned to 0) "+
			"-- the spell wrapper is reading the MANEUVER floor pair",
			spellFloored, runs)
	}
}
