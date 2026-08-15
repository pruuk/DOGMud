package combat

import (
	"math"
	"testing"
)

// U6 Task 14 — the "defended" signal.
//
// Task 10 made a defensive win land as a partially deflected hit, so res.hit
// stopped meaning "the attack won the contest". defended is the disambiguator:
// true exactly when the defence won but the swing still deals partial damage.
// Downstream consumers (momentum, progression, sounds, weapon break) key on
// hit && !defended, so a path that set it wrongly would silently hand
// deflections back their pre-U6 clean-win rewards, or strip them from real
// wins.

func TestResolveDefenseOutcome_DefenceWinSetsDefended(t *testing.T) {
	src, tgt := defenceFixture(1000)

	stdDev := 15.0
	best := defenceWinBest(stdDev*math.Sqrt2, stdDev)

	res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false)

	if !res.hit {
		t.Fatal("a defensive win must land as a partially deflected hit")
	}
	if !res.defended {
		t.Error("a defence-won non-crit outcome must carry defended == true")
	}
	if res.damageMult >= 1.0 {
		t.Errorf("a deflected swing must be mitigated, got damageMult %v", res.damageMult)
	}
}

func TestResolveDefenseOutcome_FlooredSaveIsDefended(t *testing.T) {
	src, tgt := defenceFixture(1000)

	best := defenceWinBest(1, 15.0) // margin +1 == the defence-side sentinel
	best.floored = true

	res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false)

	if !res.hit || !res.defended {
		t.Errorf("a floored save is still a defensive win dealing partial damage, got hit=%v defended=%v",
			res.hit, res.defended)
	}
}

// Every path where the attack won, fumbled, or the defence critted must leave
// defended false — those are exactly the outcomes whose downstream handling
// must NOT change.
func TestResolveDefenseOutcome_DefendedFalseOnEveryOtherPath(t *testing.T) {
	src, tgt := defenceFixture(1000)

	t.Run("attack won normally", func(t *testing.T) {
		best := defenceWinBest(-15*math.Sqrt2, 15) // negative margin == attack win
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false)
		if !res.hit || res.defended {
			t.Errorf("hit=%v defended=%v, want true/false on a clean attack win", res.hit, res.defended)
		}
	})

	t.Run("attack crit", func(t *testing.T) {
		best := defenceWinBest(-15*math.Sqrt2*3, 15)
		best.hitRoll.StdDev = 15
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false)
		if !res.crit {
			t.Fatal("fixture did not produce an attack crit")
		}
		if res.defended {
			t.Error("an attack crit must not carry defended")
		}
	})

	t.Run("defence crit", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2*3, 15)
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false)
		if !res.defenseCrit {
			t.Fatal("fixture did not produce a defence crit")
		}
		if res.defended {
			t.Error("a defence crit fully negates; it is not a deflection and must not carry defended")
		}
	})

	t.Run("attack fumble", func(t *testing.T) {
		best := defenceWinBest(-15*math.Sqrt2, 15)
		best.hitRoll.ZScore = -2.5
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false)
		if !res.fumble || res.defended {
			t.Errorf("fumble=%v defended=%v, want true/false", res.fumble, res.defended)
		}
	})

	t.Run("defence fumble", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2, 15)
		best.defRoll.ZScore = -2.5
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false)
		if !res.hit || res.defended {
			t.Errorf("hit=%v defended=%v, want true/false on a defence fumble", res.hit, res.defended)
		}
	})

	t.Run("forced crit against a sleeper", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2, 15) // the defence took the margin...
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, true)
		if !res.crit {
			t.Fatal("forceCrit must produce a crit")
		}
		if res.defended {
			t.Error("a forced crit must not carry defended")
		}
	})
}

// A crit-floor promotion turns a deflection INTO a defensive crit, and the
// declared rule is that every defensive-crit path carries defended == false.
func TestApplyCritFloors_PromotionClearsDefended(t *testing.T) {
	res := hitResolution{hit: true, defended: true, damageMult: 0.5}
	applyCritFloors(&res, &AttackResult{}, dodgeBestWon(defenceWinMargin), 0.0, 1.0)

	if !res.defenseCrit {
		t.Fatal("fixture did not promote to a defence crit")
	}
	if res.defended {
		t.Error("a floor-promoted defence crit must clear defended, as a rolled one never sets it")
	}
}
