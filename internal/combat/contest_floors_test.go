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
	// MinManeuverResistChance: 0.05, so if anyone later adds a config-loading
	// TestMain to this package, an unpinned version of this test would flip to
	// Success=false carrying the -1 sentinel margin about one run in twenty.
	pinFloors(t, 0, 0, 0, 0)

	res := RunWithManeuverFloors(10000, 1)

	if !res.Contested {
		t.Errorf("Contested = false, want true (one entry was supplied)")
	}
	if !res.Success {
		t.Errorf("Success = false, want true (attacker score 10000 vs 1, floors pinned to 0)")
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
	// reason (config.yaml ships MinSpellResistChance: 0.05).
	pinFloors(t, 0, 0, 0, 0)

	res := RunWithSpellFloors(10000, 1)

	if !res.Contested {
		t.Errorf("Contested = false, want true (one entry was supplied)")
	}
	if !res.Success {
		t.Errorf("Success = false, want true (attacker score 10000 vs 1, floors pinned to 0)")
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
