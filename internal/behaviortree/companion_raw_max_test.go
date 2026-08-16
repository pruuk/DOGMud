package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// U7b Task 11. hpPercent is the archer archetype's only health gate, and its
// single caller (actKeepDistance) passes the acting mob itself. Dividing by the
// raw maximum makes a reserved companion read as permanently wounded at a
// completely full pool, so a reserved archer refuses to kite forever: the kite
// branch bails when it judges itself too hurt to disengage safely.
//
// U7 left this read alone on the premise that mobs cannot carry a reservation.
// GetPoolReservation has no IsMob gate, and companions wearing enchanted gear
// reserve on production today.

// reservedAtCeiling builds a character reserving `pct` of a 100-point health
// pool and sitting at its FULL reachable health, plus an identical unreserved
// character genuinely down to that same absolute value. Their current health is
// equal by construction, so only the denominator can separate them.
func reservedAtCeiling(t *testing.T, pct float64) (reserved, depleted *characters.Character) {
	t.Helper()

	const specId = 999960
	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId:           specId,
		Name:             "leeching collar",
		Type:             items.Neck,
		Subtype:          items.Wearable,
		ReserveHealthPct: pct,
	})

	const rawMax = 100
	reachable := rawMax - int(rawMax*pct)

	build := func(equip bool) *characters.Character {
		c := characters.New()
		if equip {
			c.Equipment.Neck = items.New(specId)
		}
		c.Validate()
		// Set the pool AFTER Validate: Validate derives the maximum from .Base
		// and would overwrite this.
		c.HealthMax.Value = rawMax
		c.Health = reachable
		return c
	}

	reserved, depleted = build(true), build(false)

	if got := reserved.EffectivePoolMax(characters.PoolHealth); got != reachable {
		t.Fatalf("fixture: reserved EffectivePoolMax(health) = %d, want %d; the item spec did not register", got, reachable)
	}
	if got := depleted.EffectivePoolMax(characters.PoolHealth); got != rawMax {
		t.Fatalf("fixture: unreserved EffectivePoolMax(health) = %d, want %d", got, rawMax)
	}
	return reserved, depleted
}

func TestHpPercentReadsTheReachableHealthPool(t *testing.T) {
	reserved, depleted := reservedAtCeiling(t, 0.66)

	full := hpPercent(reserved)
	worn := hpPercent(depleted)

	if full < 99 {
		t.Errorf("hpPercent reports %.1f%% for a companion reserved at the U7b ceiling "+
			"and sitting at its FULL reachable health, want 100%%. That is the raw max "+
			"in the denominator instead of EffectivePoolMax", full)
	}
	if worn >= 99 {
		t.Fatalf("fixture: a genuinely wounded character reports %.1f%%; the fixture is "+
			"not exercising anything", worn)
	}
	if full <= worn {
		t.Errorf("hpPercent cannot tell a rested reserved companion (%.1f%%) from a "+
			"genuinely wounded one (%.1f%%)", full, worn)
	}
}
