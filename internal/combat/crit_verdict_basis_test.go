package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dice"
)

// #481: the AttackCrit debug line used to print the attacker's self-relative
// z-score, which the crit verdict has not read since chunk 5.11d (it reads
// the normalised opposed margin when a defence was rolled). The resolution
// now carries the value and basis the verdict used, and the line logs those.
func TestResolveDefenseOutcome_RecordsTheCritVerdictBasis(t *testing.T) {
	src, tgt := defenceFixture(1000)

	t.Run("margin", func(t *testing.T) {
		// attackWinBest: margin -15*sqrt2 over stdDev 15 -> normalised z 1.0,
		// while the self-relative hitRoll z is 0.
		res := resolveDefenseOutcomeInner(&AttackResult{}, attackWinBest(), src, tgt, 0.8, false, false, false)
		if !res.crit {
			t.Fatalf("precondition: a 1.0 margin z over a 0.8 bar must crit")
		}
		if res.critBasis != "margin" || math.Abs(res.critZ-1.0) > 1e-9 {
			t.Errorf("got basis=%q critZ=%v, want margin 1.0", res.critBasis, res.critZ)
		}
	})

	t.Run("selfz", func(t *testing.T) {
		best := bestDefenseResult{hitRoll: dice.RollResult{Value: 100, ZScore: 2.5}}
		res := resolveDefenseOutcomeInner(&AttackResult{}, best, src, tgt, 2.0, false, false, false)
		if !res.crit {
			t.Fatalf("precondition: an undefended 2.5 self z over a 2.0 bar must crit")
		}
		if res.critBasis != "selfz" || res.critZ != 2.5 {
			t.Errorf("got basis=%q critZ=%v, want selfz 2.5", res.critBasis, res.critZ)
		}
	})

	t.Run("forced", func(t *testing.T) {
		res := resolveDefenseOutcomeInner(&AttackResult{}, attackWinBest(), src, tgt, 5.0, false, true, false)
		if !res.crit {
			t.Fatalf("precondition: a forced crit must crit")
		}
		if res.critBasis != "forced" {
			t.Errorf("got basis=%q, want forced", res.critBasis)
		}
	})
}
