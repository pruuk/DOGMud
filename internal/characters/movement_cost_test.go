package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
)

// travellerAt returns a fresh character loaded to the given fraction of carry
// capacity. Skills are left at the rank-1 floor every character starts with,
// which is where 99 of the 108 live characters actually sit for `search`.
//
// itemId must be unique per call: the item spec registry is package-global and
// shared across the whole test binary.
func travellerAt(t *testing.T, itemId int, fraction float64) *Character {
	t.Helper()

	c := New()
	c.Stats.Strength.Base = 100
	c.Stats.Strength.Recalculate()
	c.Validate()
	if fraction > 0 {
		loadToFraction(t, c, itemId, fraction)
	}
	return c
}

// Ordinary travel must not get MORE expensive. The whole trade U7 Task 8 makes
// is that a realistically laden traveller pays a little less than the old flat
// two, and that saving is what pays for the steep price near carry capacity.
//
// A "realistic" load is 35% of capacity: the shared curve reads 1.233 there,
// against the old inline term's 1.0 (which stayed 1.0 until the actor EXCEEDED
// capacity and so priced nothing for anyone not deliberately overloaded).
func TestOrdinaryTravelCostsNoMoreThanTheOldFlatTwo(t *testing.T) {
	c := travellerAt(t, 99820, 0.35)

	if got := c.GetMovementStaminaCost(1.0); got > 2 {
		t.Fatalf("ordinary travel at 35%% of capacity costs %d, above the old "+
			"flat 2; the base drop to 0.5 is meant to make normal travel cheaper, "+
			"not dearer", got)
	}
}

// Travel at carry capacity is dearer than travel at a realistic load. This is
// the assertion the old curve could not make: at capacity the old term was still
// exactly 1.0, so hauling a full pack across a continent cost the same as
// strolling empty-handed.
func TestTravelAtCapacityCostsMoreThanARealisticLoad(t *testing.T) {
	realistic := travellerAt(t, 99821, 0.35)
	laden := travellerAt(t, 99822, 1.0)

	light := realistic.GetMovementStaminaCost(1.0)
	heavy := laden.GetMovementStaminaCost(1.0)

	if heavy <= light {
		t.Fatalf("a traveller at carry capacity pays %d and one at 35%% pays %d; "+
			"the shared encumbrance curve is not reaching the movement price",
			heavy, light)
	}
}

// Terrain survives U7 as its own multiplier, applied to the base exactly as it
// was before. Rough ground is dearer than a road at the same load.
func TestRoughTerrainCostsMoreThanNormalTerrain(t *testing.T) {
	c := travellerAt(t, 99823, 0.35)

	normal := c.GetMovementStaminaCost(1.0)
	rough := c.GetMovementStaminaCost(2.0)

	if rough <= normal {
		t.Fatalf("rough terrain costs %d and normal terrain %d at the same load; "+
			"terrain must still be priced", rough, normal)
	}
}

// No move is ever free. Movement cannot bank a fractional remainder the way a
// defence can -- go.go spends whole stamina -- so a sub-1.0 cost with no floor
// would round away to nothing and make travel free for an unladen character.
func TestEveryMoveCostsAtLeastTheFloor(t *testing.T) {
	floor := int(configs.GetBalanceConfig().MovementCostFloor)
	if floor < 1 {
		t.Fatalf("MovementCostFloor validated to %d; a floor below 1 makes moves free", floor)
	}

	// An unladen character on frictionless ground: every factor is at its
	// cheapest and terrain is degenerate. Only the floor stands between this and
	// a free move.
	c := travellerAt(t, 99824, 0)
	for _, terrain := range []float64{0, 0.25, 0.5, 1.0} {
		if got := c.GetMovementStaminaCost(terrain); got < floor {
			t.Errorf("terrain %.2f priced at %d, below the floor of %d", terrain, got, floor)
		}
	}
}

// A practised traveller never pays MORE than a novice at the same load.
//
// Asserted as "no more", not "strictly less", because at ordinary loads the
// floor genuinely swallows the difference: at base 0.5 a rank-1 character
// computes 0.676 and a capped-skill one 0.247, and both floor to 1. That is
// expected, not a bug -- the inverse-skill term only bites once load or terrain
// has pushed the cost clear of the floor. The at-capacity half of this test is
// what proves the term is actually wired in rather than silently dropped, which
// a floored comparison alone can never show.
func TestSkilledTravellerNeverPaysMoreThanANovice(t *testing.T) {
	capRank := int(configs.GetBalanceConfig().CostSkillCapRank)
	spec := costs.SpecFor(costs.ActionMove)

	novice := travellerAt(t, 99825, 0.35)
	master := travellerAt(t, 99826, 0.35)
	master.SetSkill(string(spec.Skill), capRank)

	if master.GetMovementStaminaCost(1.0) > novice.GetMovementStaminaCost(1.0) {
		t.Errorf("at a realistic load the master pays %d and the novice %d; skill "+
			"must never make travel dearer",
			master.GetMovementStaminaCost(1.0), novice.GetMovementStaminaCost(1.0))
	}

	// Above the floor, where the discount is visible.
	ladenNovice := travellerAt(t, 99827, 1.0)
	ladenMaster := travellerAt(t, 99828, 1.0)
	ladenMaster.SetSkill(string(spec.Skill), capRank)

	cheap := ladenMaster.GetMovementStaminaCost(1.0)
	dear := ladenNovice.GetMovementStaminaCost(1.0)
	if cheap >= dear {
		t.Errorf("at carry capacity the master pays %d and the novice %d; the "+
			"inverse-skill term is not reaching the movement price", cheap, dear)
	}
}

// The three headline magnitudes the Task 8 design was signed off on, stated as
// numbers so a config change that moves them cannot pass silently.
//
// Derived from the shared curves rather than hardcoded, so this tracks a knob
// retune instead of breaking on one; the integer column is the player-visible
// result and IS pinned.
func TestMovementCostMagnitudes(t *testing.T) {
	bal := configs.GetBalanceConfig()
	base := float64(bal.MovementBaseStaminaCost)
	skill := costs.SkillCostMultiplier(1) // the universal rank-1 floor

	cases := []struct {
		name     string
		fraction float64
		terrain  float64
		itemId   int
		wantInt  int
	}{
		// 0.5 x 1.233 x 1.096 = 0.676 -> 1, against a flat 2 before U7.
		{"realistic newcomer, normal terrain", 0.35, 1.0, 99829, 1},
		// 0.5 x 5.0 x 1.096 = 2.74 -> 3, against a flat 2 before U7.
		{"at carry capacity, normal terrain", 1.0, 1.0, 99830, 3},
		// 0.676 x 2 = 1.35 -> 2, against a flat 4 before U7.
		{"realistic newcomer, rough terrain", 0.35, 2.0, 99831, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := travellerAt(t, tc.itemId, tc.fraction)

			want := base * tc.terrain *
				costs.EncumbranceMultiplier(c.GetCarriedWeight(), c.CarryCapacity()) *
				skill

			if got := c.GetMovementStaminaCost(tc.terrain); got != tc.wantInt {
				t.Errorf("cost is %d, want %d (composed %.3f)", got, tc.wantInt, want)
			}
			if math.Ceil(want) != float64(tc.wantInt) {
				t.Errorf("composed cost %.3f no longer ceils to the expected %d; a "+
					"cost knob has moved and the Task 8 magnitudes need re-signing",
					want, tc.wantInt)
			}
		})
	}
}
