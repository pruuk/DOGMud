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
		t.Fatalf("ordinary travel at 35%% of capacity costs %.4f, above the old "+
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
		t.Fatalf("a traveller at carry capacity pays %.4f and one at 35%% pays %.4f; "+
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
		t.Fatalf("rough terrain costs %.4f and normal terrain %.4f at the same load; "+
			"terrain must still be priced", rough, normal)
	}
}

// No move is ever free, but "not free" no longer means "at least a whole point".
// MovementCostFloor is deleted: a cheap move is BANKED, so it costs nothing on
// the round it happens and a whole point some rooms later. Charging a point up
// front instead is exactly the flattening the floor used to cause.
//
// This is one of the two tests that bite if movement stops banking. Restore the
// old ceiling and the first cheap move charges a point, and the first assertion
// below fails.
func TestACheapMoveIsBankedNotFree(t *testing.T) {
	c := travellerAt(t, 99824, 0)
	c.StaminaMax.Base = 100
	c.StaminaMax.Recalculate()
	c.Stamina = 100

	// An unladen traveller on easy ground: every factor is at its cheapest, and
	// the price is a fraction of a point.
	cost := c.GetMovementStaminaCost(0.25)
	if cost <= 0 {
		t.Fatalf("a move is priced at %v; no move may ever be free", cost)
	}
	if cost >= 1 {
		t.Fatalf("the cheapest move the game can price is %v, not a fraction; the "+
			"cost is being rounded up somewhere and the encumbrance curve cannot "+
			"express itself on top of a base below 1", cost)
	}

	if !c.ApplyCostFloatOrRefuse(PoolStamina, cost) {
		t.Fatalf("a %v move was refused with a full pool", cost)
	}
	if c.Stamina != 100 {
		t.Errorf("one cheap move took %d stamina; a sub-1 cost must be banked, not "+
			"charged as a whole point", 100-c.Stamina)
	}

	// Banked, not forgiven: enough of them do add up to real stamina.
	for i := 0; i < 20; i++ {
		if !c.ApplyCostFloatOrRefuse(PoolStamina, c.GetMovementStaminaCost(0.25)) {
			t.Fatalf("cheap move %d was refused with %d stamina in hand", i+2, c.Stamina)
		}
	}
	if c.Stamina >= 100 {
		t.Errorf("21 cheap moves cost nothing at all; the remainder is being " +
			"discarded rather than banked, which makes travel free")
	}
}

// THE HEADLINE ASSERTION FOR THE BANKING CHANGE. A light traveller crossing
// twenty rooms must spend markedly less than twenty stamina.
//
// The measured price is composed from the shared curves rather than read back
// out of the function under test, so a regression that re-ceils the per-move
// cost cannot move the expectation along with it and pass anyway.
func TestMovementBanksItsFractionalRemainder(t *testing.T) {
	const (
		moves = 20
		pool  = 500
	)

	c := travellerAt(t, 99832, 0.09)
	c.StaminaMax.Base = pool
	c.StaminaMax.Recalculate()
	c.Stamina = pool

	perMove := float64(configs.GetBalanceConfig().MovementBaseStaminaCost) *
		costs.EncumbranceMultiplier(c.GetCarriedWeight(), c.CarryCapacity()) *
		costs.SkillCostMultiplier(1)

	for i := 0; i < moves; i++ {
		if !c.ApplyCostFloatOrRefuse(PoolStamina, c.GetMovementStaminaCost(1.0)) {
			t.Fatalf("move %d was refused with %d stamina in hand", i+1, c.Stamina)
		}
	}

	spent := pool - c.Stamina
	want := int(math.Floor(perMove * moves))

	if spent != want {
		t.Errorf("%d rooms at a light load cost %d stamina, want %d (%.4f a room, "+
			"banked); movement is no longer charging its true fractional price",
			moves, spent, want, perMove)
	}
	if spent >= moves {
		t.Errorf("%d rooms at a light load cost %d stamina, one or more per room; "+
			"the remainder is being rounded up instead of banked, which is what "+
			"flattened the encumbrance curve into a single step", moves, spent)
	}
}

// A refused move must leave the carry bank exactly as it found it. Movement
// still REFUSES when unaffordable (U5b-2, the gate that makes flee the only
// player-initiated disengage in combat), and the refusal is decided before
// anything is written, so hammering a direction you cannot afford cannot quietly
// run up a debt that lands on the first move you CAN afford.
func TestARefusedMoveBanksNothing(t *testing.T) {
	c := travellerAt(t, 99834, 1.32)
	c.StaminaMax.Base = 100
	c.StaminaMax.Recalculate()
	c.Stamina = 1

	cost := c.GetMovementStaminaCost(1.0)
	if cost < 2 {
		t.Fatalf("fixture prices a crushed move at %v; it must exceed the 1 stamina "+
			"on hand for this test to exercise refusal at all", cost)
	}

	for i := 0; i < 5; i++ {
		if c.ApplyCostFloatOrRefuse(PoolStamina, cost) {
			t.Fatalf("attempt %d: a crushed traveller with 1 stamina was allowed to "+
				"move; movement must refuse rather than pay what it can", i+1)
		}
	}
	if c.Stamina != 1 {
		t.Errorf("pool is %d after five refused moves, want 1; a refusal must not "+
			"charge anything", c.Stamina)
	}
	if got := c.costCarry[PoolStamina]; got != 0 {
		t.Errorf("carry is %v after five refused moves, want 0; a refused move must "+
			"not accumulate debt for a later affordable one to inherit", got)
	}

	// And once they can afford it, they pay the real price and nothing more.
	c.Stamina = 100
	if !c.ApplyCostFloatOrRefuse(PoolStamina, cost) {
		t.Fatalf("the same move was refused with a full pool")
	}
	if want := 100 - int(math.Floor(cost)); c.Stamina != want {
		t.Errorf("pool is %d after the first affordable move, want %d; the five "+
			"refusals left debt behind", c.Stamina, want)
	}
}

// A practised traveller never pays MORE than a novice at the same load, and
// with the floor gone the discount is now visible at ORDINARY loads too, not
// only near capacity. That is the second-order win from banking: 0.676 against
// 0.247 used to be two ways of writing "1 stamina", and is now a traveller who
// covers nearly three times the ground on the same pool.
func TestSkilledTravellerNeverPaysMoreThanANovice(t *testing.T) {
	capRank := int(configs.GetBalanceConfig().CostSkillCapRank)
	spec := costs.SpecFor(costs.ActionMove)

	novice := travellerAt(t, 99825, 0.35)
	master := travellerAt(t, 99826, 0.35)
	master.SetSkill(string(spec.Skill), capRank)

	if master.GetMovementStaminaCost(1.0) >= novice.GetMovementStaminaCost(1.0) {
		t.Errorf("at a realistic load the master pays %.4f and the novice %.4f; with "+
			"the cost floor gone the discount must be visible at ordinary loads, "+
			"not only near capacity",
			master.GetMovementStaminaCost(1.0), novice.GetMovementStaminaCost(1.0))
	}

	// Above the floor, where the discount is visible.
	ladenNovice := travellerAt(t, 99827, 1.0)
	ladenMaster := travellerAt(t, 99828, 1.0)
	ladenMaster.SetSkill(string(spec.Skill), capRank)

	cheap := ladenMaster.GetMovementStaminaCost(1.0)
	dear := ladenNovice.GetMovementStaminaCost(1.0)
	if cheap >= dear {
		t.Errorf("at carry capacity the master pays %.4f and the novice %.4f; the "+
			"inverse-skill term is not reaching the movement price", cheap, dear)
	}
}

// The measured cost per room across the four loads the in-game playtest
// reported, on road and on rough ground, stated as numbers so a config change
// that moves them cannot pass silently.
//
// The playtest that motivated the banking change found a single step with flat
// shoulders: light and heavy both cost 1 a room, overburdened and crushed both
// cost 2 to 3. THIS TABLE IS WHAT REPLACED IT, and the point of pinning it is
// that the eight rows are now eight distinct prices. The `wantCost` column is a
// literal on purpose: composing the expectation from the same curves the code
// uses would let a knob retune move both sides together and pass.
//
// The cross-check underneath composes the price from the shared curves anyway,
// so a failure says WHICH knob moved rather than only that something did.
func TestMovementCostMagnitudes(t *testing.T) {
	bal := configs.GetBalanceConfig()
	base := float64(bal.MovementBaseStaminaCost)
	skill := costs.SkillCostMultiplier(1) // the universal rank-1 floor

	const tolerance = 1e-6

	cases := []struct {
		name     string
		fraction float64
		terrain  float64
		itemId   int
		wantCost float64
	}{
		// Road. 0.5 x encumbrance x 1.096.
		{"light, road", 0.09, 1.0, 99829, 0.580880},
		{"heavy, road", 0.66, 1.0, 99830, 0.789120},
		{"overburdened, road", 0.88, 1.0, 99831, 1.819360},
		{"crushed, road", 1.32, 1.0, 99835, 2.740000},

		// Rough ground doubles the base, and terrain lives INSIDE the base by
		// design, so every row is exactly twice its road counterpart.
		{"light, rough", 0.09, 2.0, 99836, 1.161760},
		{"heavy, rough", 0.66, 2.0, 99837, 1.578240},
		{"overburdened, rough", 0.88, 2.0, 99838, 3.638720},
		{"crushed, rough", 1.32, 2.0, 99839, 5.480000},
	}

	seen := make(map[float64]string, len(cases))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := travellerAt(t, tc.itemId, tc.fraction)

			got := c.GetMovementStaminaCost(tc.terrain)
			if math.Abs(got-tc.wantCost) > tolerance {
				t.Errorf("cost is %.6f a room, want %.6f", got, tc.wantCost)
			}

			composed := base * tc.terrain *
				costs.EncumbranceMultiplier(c.GetCarriedWeight(), c.CarryCapacity()) *
				skill
			if math.Abs(composed-tc.wantCost) > tolerance {
				t.Errorf("the shared curves now compose to %.6f, not the signed-off "+
					"%.6f; a cost knob has moved and these magnitudes need re-signing",
					composed, tc.wantCost)
			}
		})

		if prev, dup := seen[tc.wantCost]; dup {
			t.Errorf("%q and %q are pinned to the same price %.6f; the whole point "+
				"of banking the remainder is that these loads are no longer the "+
				"same single step with flat shoulders", prev, tc.name, tc.wantCost)
		}
		seen[tc.wantCost] = tc.name
	}
}
