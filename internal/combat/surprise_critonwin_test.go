package combat

import (
	"math"
	"testing"
)

// attackWinBest is a settled ATTACK win below the crit bar, so only critOnWin
// can make it crit.
//
// hitRoll.StdDev is set deliberately. defenceWinBest leaves it at zero, which
// makes normalizedAttackMargin return ok == false and sends the crit check down
// the legacy self-relative fallback rather than the margin path production
// actually uses. With StdDev == defStdDev the normalised attack margin is
// -(-15*sqrt2) / (15*sqrt2) == 1.0 -- a real, margin-derived z, comfortably
// under the 2.0 bar. So the fixture is sub-crit on the PATH IT CLAIMS TO BE ON,
// and nothing but critOnWin can promote it.
func attackWinBest() bestDefenseResult {
	best := defenceWinBest(-15*math.Sqrt2, 15)
	best.hitRoll.StdDev = 15
	return best
}

func TestCritOnWin_UpgradesWinButNeverRescuesALoss(t *testing.T) {
	src, tgt := defenceFixture(1000)

	t.Run("clean attack win becomes a crit", func(t *testing.T) {
		res := resolveDefenseOutcomeCore(&AttackResult{}, attackWinBest(), src, tgt, 2.0, false, false, true)
		if !res.hit || res.defended {
			t.Fatalf("precondition: expected a clean attack win")
		}
		if !res.crit {
			t.Fatalf("critOnWin must upgrade a won contest")
		}
	})

	t.Run("defence win stays a defence win", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2, 15) // positive == defence won
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, true)
		if res.crit {
			t.Fatalf("critOnWin must never rescue a lost contest")
		}
	})

	t.Run("critOnWin false is unchanged behaviour", func(t *testing.T) {
		res := resolveDefenseOutcomeCore(&AttackResult{}, attackWinBest(), src, tgt, 2.0, false, false, false)
		if res.crit {
			t.Fatalf("an ordinary win must not crit merely from winning")
		}
	})

	t.Run("a FLOORED win must not crit", func(t *testing.T) {
		best := attackWinBest()
		best.floored = true
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, true)
		if res.crit {
			t.Fatalf("a sentinel margin must never be promoted to a crit")
		}
	})

	// The guard's `best.margin <= 0` term is the only one that is not implied by
	// the others, and this is the case that separates it. A defence FUMBLE exits
	// the inner resolver early, BEFORE attackWon is ever computed, and hands back
	// hit == true / defended == false / fumble == false on a swing the attack
	// LOST on margin. Without the margin term the guard would fire here and turn
	// the defender's stumble into an opening-strike assassination.
	t.Run("a defence FUMBLE the attack lost on margin must not crit", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2, 15) // positive == the DEFENCE took the margin
		best.defRoll.ZScore = -3.0                // ... and then fumbled it away

		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, true)

		if !res.hit || res.defended || res.fumble {
			t.Fatalf("precondition: expected the defence-fumble exit "+
				"(hit=true, defended=false, fumble=false), got hit=%v defended=%v fumble=%v",
				res.hit, res.defended, res.fumble)
		}
		if best.margin <= 0 {
			t.Fatalf("precondition: the ATTACK must have lost the margin, got %v", best.margin)
		}
		if res.crit {
			t.Fatalf("critOnWin must not promote a swing the attack lost on margin " +
				"and won only because the defender fumbled")
		}
	})
}
