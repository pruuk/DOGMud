package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// U7b Task 11. Reservation is not woundedness, and it is not a player-only
// concern.
//
// The three self-side reads covered here divided current health by the RAW
// maximum while RecalculateStats had already clamped that current value to
// max minus reservation. A companion sitting at its ceiling therefore scored
// itself as badly hurt and picked its tactics for a crisis it was not in.
//
// U7 left these alone on the premise that mobs cannot carry a reservation.
// That premise is false: GetPoolReservation has no IsMob gate, and Meirok's
// two golems wear enchanted gear reserving their own health and conviction on
// the live save. Every one of these tests goes green on a revert to
// HealthMax.Value only if the reserved actor and the genuinely wounded one are
// scored identically, which is exactly the claim the raw-max read makes.
//
// Fixture note: reservedAndDepleted (effective_pool_penalty_test.go) builds a
// pair whose CURRENT health is equal by construction, so the only thing that
// can separate them is the denominator.

// asMob wraps a character as a mob instance. The three scorers take *mobs.Mob
// and read nothing off it but the embedded Character.
func asMob(c *characters.Character) *mobs.Mob {
	m := &mobs.Mob{}
	m.Character = *c
	return m
}

// ai.go ScoreGrapple. Below 20% health the mob subtracts 50 from the grapple
// score, which at a base of 50 zeroes the move outright: a reserved companion
// would never grapple again, however rested it was.
//
// 90% reservation, not the U7b ceiling of 66%: the gate is at 20%, so 66% (a
// raw reading of 34%) does not reach it. The ceiling is exercised by the drain
// test below.
func TestScoreGrappleReadsTheCompanionsReachableHealth(t *testing.T) {
	reserved, depleted := reservedAndDepleted(t, characters.PoolHealth, 0.90)

	// characters.New() RANDOMISES stats, and ScoreGrapple pays +20 when the mob
	// out-muscles the target. Both mobs face the SAME target, so that term is
	// identical for the pair and cancels in the difference; asserting on the
	// difference rather than on either absolute score keeps this deterministic.
	target := characters.New()
	target.Validate()
	target.HealthMax.Value = 100
	target.Health = 100

	full := ScoreGrapple(asMob(reserved), target)
	worn := ScoreGrapple(asMob(depleted), target)

	if full <= worn {
		t.Errorf("ScoreGrapple scores a fully rested 90%%-reserved companion %d against "+
			"%d for a genuinely near-dead one. They must differ: the reserved companion "+
			"is at its REACHABLE maximum. Equal scores mean ScoreGrapple divides by "+
			"HealthMax.Value instead of EffectivePoolMax", full, worn)
	}
	if full-worn != 50 {
		t.Errorf("the gap between the rested reserved companion (%d) and the genuinely "+
			"near-dead one (%d) is %d, want exactly the 50-point low-health penalty; "+
			"only the wounded one may pay it", full, worn, full-worn)
	}
}

// ai.go ScoreDrain. The bonus fires below 60% health, which the U7b ceiling of
// 66% reservation reaches on its own: a capped companion reads as permanently
// at 34% and permanently prioritises the self-heal drain.
func TestScoreDrainReadsTheCompanionsReachableHealth(t *testing.T) {
	reserved, depleted := reservedAndDepleted(t, characters.PoolHealth, 0.66)

	target := characters.New()
	target.Validate()

	full := ScoreDrain(asMob(reserved), target)
	worn := ScoreDrain(asMob(depleted), target)

	if full >= worn {
		t.Errorf("ScoreDrain scores a fully rested companion reserved at the U7b ceiling "+
			"%d against %d for a genuinely hurt one. The rested one must NOT take the "+
			"hurt-attacker bonus. Equal scores mean ScoreDrain divides by "+
			"HealthMax.Value instead of EffectivePoolMax", full, worn)
	}
}

// ai.go preferredSpell. The self-heal branch fires below 30% health, so a
// reserved caster companion would burn every round healing a pool that is
// already as full as it can get.
func TestPreferredSpellReadsTheCompanionsReachableHealth(t *testing.T) {
	defer spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"heal": {SpellId: "heal", Name: "Heal", Cost: 5, EffectType: "heal"},
	})()

	reserved, depleted := reservedAndDepleted(t, characters.PoolHealth, 0.90)

	full := asMob(reserved)
	worn := asMob(depleted)
	for _, m := range []*mobs.Mob{full, worn} {
		m.Character.SpellBook = map[string]int{"heal": 1}
		m.Character.Conviction = 100
	}

	if got := preferredSpell(worn); got != "heal" {
		t.Fatalf("fixture: a genuinely near-dead caster chose %q, want \"heal\"; the "+
			"self-heal branch is not reachable and this test would prove nothing", got)
	}
	if got := preferredSpell(full); got == "heal" {
		t.Errorf("preferredSpell made a fully rested 90%%-reserved companion cast heal on " +
			"itself. Its pool is already at its REACHABLE maximum. This means " +
			"preferredSpell divides by HealthMax.Value instead of EffectivePoolMax")
	}
}
