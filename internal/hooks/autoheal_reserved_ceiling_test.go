package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #285: the round regen tick fills a reserved pool to the reserved ceiling
// (raw max less reservation) and no further, for players and mobs alike.
// Before the fix it filled to the raw max and the next RecalculateStats pulled
// it back, so the prompt and GMCP showed the inflated value in between.

const autoHealReservingItemId = 999286

// reserveAllPools gives c a worn neck item reserving 25% of every pool, with
// raw maxes of 400 and pools one point below their ceilings (300, 300, 300)
// so any regen at all reaches the ceiling.
func reserveAllPools(t *testing.T, c *characters.Character) {
	t.Helper()
	c.HealthMax.Base = 400
	c.StaminaMax.Base = 400
	c.ConvictionMax.Base = 400
	c.Equipment.Neck = items.New(autoHealReservingItemId)
	c.Validate()
	for _, p := range []characters.Pool{characters.PoolHealth, characters.PoolStamina, characters.PoolConviction} {
		ceiling := c.PoolCeiling(p)
		if ceiling >= 400 || ceiling < 2 {
			t.Fatalf("setup: %s ceiling %d is not reserved below the raw 400", p, ceiling)
		}
	}
	c.Health = c.PoolCeiling(characters.PoolHealth) - 1
	c.Stamina = c.PoolCeiling(characters.PoolStamina) - 1
	c.Conviction = c.PoolCeiling(characters.PoolConviction) - 1
}

func assertAtOrBelowCeiling(t *testing.T, who string, c *characters.Character) {
	t.Helper()
	for _, p := range []characters.Pool{characters.PoolHealth, characters.PoolStamina, characters.PoolConviction} {
		if got, ceiling := c.PoolValue(p), c.PoolCeiling(p); got > ceiling {
			t.Errorf("%s %s regenerated to %d, past the reserved ceiling %d", who, p, got, ceiling)
		}
	}
	// Health regen is floored at 1 a tick out of combat, so it must land
	// exactly on the ceiling.
	if got, ceiling := c.Health, c.PoolCeiling(characters.PoolHealth); got != ceiling {
		t.Errorf("%s health = %d after a regen tick, want the reserved ceiling %d", who, got, ceiling)
	}
}

func TestAutoHeal_RespectsReservedCeiling(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		autoHealReservingItemId: {
			ItemId:               autoHealReservingItemId,
			Name:                 "grasping torc",
			Type:                 items.Neck,
			ReserveHealthPct:     0.25,
			ReserveStaminaPct:    0.25,
			ReserveConvictionPct: 0.25,
		},
	})()

	u1 := users.GetByUserId(1)
	u1.Character.EndAggro()
	reserveAllPools(t, u1.Character)

	mob := mobs.GetInstance(100)
	mob.Character.EndAggro()
	reserveAllPools(t, &mob.Character)

	AutoHeal(events.NewRound{RoundNumber: 30})

	assertAtOrBelowCeiling(t, "player", u1.Character)
	assertAtOrBelowCeiling(t, "mob", &mob.Character)
}
