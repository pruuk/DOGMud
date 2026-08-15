package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// runBestOfAllDefense discarded the contest.Result, so the floored flag never
// reached resolution. The new order needs it: a floored outcome carries the
// +-1 sentinel margin rather than a real one, and must never be promoted to a
// crit.
//
// Uses defenceFixture (defense_affordability_test.go) rather than building a
// deliberately lopsided pair: RunWithFloors can flip EITHER outcome with
// probability ContestFloor regardless of who is favoured, so an evenly
// matched fixture is enough to observe a flip across enough rolls.
func TestRunBestOfAllDefense_CarriesFlooredFlag(t *testing.T) {
	attacker, defender := defenceFixture(100)

	sawFloored := false
	for i := 0; i < 5000; i++ {
		result := &AttackResult{}
		ctx := combatContext{sourceCanSee: true, targetCanSee: true}

		best := runBestOfAllDefense(result, attacker, defender,
			[]string{characters.DefenseDodge}, 100.0, false, ctx)

		if best.floored {
			sawFloored = true
			break
		}
	}
	if !sawFloored {
		t.Fatal("the contest floor did not surface as best.floored across 5000 rolls; " +
			"runBestOfAllDefense may still be discarding contest.Result.Floored")
	}
}
