package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// The dark-combat bucketing (M6 slice 2) reads a per-swing defence result.
// AttackResult.DefenseUsed is set on a defensive win and never cleared, so it
// cannot answer "did THIS swing's defence win". SwingEvent.Defended does.
//
// This drives the real composers: calculateCombat over a bare-handed attacker
// (one fist per hand, so a multi-swing round) until it has produced a round
// whose first swing was defended and whose second landed clean. In that round
// the sticky DefenseUsed is non-empty for the WHOLE round, while the per-swing
// Defended differs between the swings; a Defended that were copied from
// DefenseUsed would read {true, true}.
func TestSwingEvent_DefendedIsPerSwingThroughCalculateCombat(t *testing.T) {
	atk, def := swingDefencePopulationCombatants()

	const maxRounds = 5000
	for i := 0; i < maxRounds; i++ {
		atk.Stamina, def.Stamina, def.Health = 1000, 500, 5000
		plan := buildAttackPlan(atk, def)
		res := calculateCombat(atk, def, User, Mob, plan, combatContext{
			sourceSight: messaging.SightFull, targetSight: messaging.SightFull,
		})
		if len(res.SwingEvents) < 2 {
			continue
		}
		anyDefended := false
		for _, se := range res.SwingEvents {
			anyDefended = anyDefended || se.Defended
		}
		if anyDefended && res.DefenseUsed == "" {
			t.Fatalf("round %d: a swing was Defended but AttackResult.DefenseUsed is empty", i)
		}
		first, second := res.SwingEvents[0], res.SwingEvents[1]
		if first.Defended && !second.Defended && second.Hit {
			if res.DefenseUsed == "" {
				t.Fatalf("round %d: DefenseUsed is empty although swing 1 was defended", i)
			}
			return
		}
	}
	t.Fatalf("no round in %d produced swing 1 defended and swing 2 a clean hit; the fixture no longer exercises a mixed round", maxRounds)
}

func TestHitResolution_DefenceWonFeedsDefended(t *testing.T) {
	cases := []struct {
		r    hitResolution
		want bool
	}{
		{hitResolution{}, false},
		{hitResolution{defended: true}, true},
		{hitResolution{defenseCrit: true}, true},
	}
	for _, c := range cases {
		if got := swingEventFor(c.r).Defended; got != c.want {
			t.Fatalf("swingEventFor(%+v).Defended = %v, want %v", c.r, got, c.want)
		}
	}
}
