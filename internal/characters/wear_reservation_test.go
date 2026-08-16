package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// seedMediumSpecies is required by every test here, and is not optional dressing.
// Wear calls HandsRequired, which dereferences species.GetSpecies(c.SpeciesId)
// without a nil guard; this package loads no data files, so without a seeded
// species every one of these tests panics before it reaches the ceiling check.
// New() starts a character at species 0.
func seedMediumSpecies() func() {
	return species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "human", Size: species.Medium},
	})
}

// An equip that would carry reservation past the cap is REFUSED (D3), and the
// character is left exactly as they were: the item is not on the body, and
// nothing was displaced.
func TestWear_RefusesABreachingEquip(t *testing.T) {
	defer seedMediumSpecies()()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999950: {ItemId: 999950, Name: "hungry collar", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.60},
		999951: {ItemId: 999951, Name: "hungry belt", Type: items.Belt, Subtype: items.Wearable, ReserveStaminaPct: 0.30},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999950) // 60 against a 66 cap
	c.Validate()

	returned, worn, reason := c.Wear(items.New(999951)) // +30 would make 90
	if worn {
		t.Fatalf("a breaching equip must be refused, not worn")
	}
	if len(returned) != 0 {
		t.Errorf("a refused equip must displace nothing, got %d items back", len(returned))
	}
	if c.Equipment.Belt.ItemId != 0 {
		t.Errorf("a refused equip must leave the slot empty, found item %d", c.Equipment.Belt.ItemId)
	}
	if c.Equipment.Neck.ItemId != 999950 {
		t.Errorf("a refused equip must leave existing gear untouched")
	}
	if !strings.Contains(strings.ToLower(reason), "reserve") {
		t.Errorf("the refusal must name reservation as the cause, got %q", reason)
	}
	if strings.ContainsAny(reason, "0123456789") {
		t.Errorf("a player-facing refusal must carry no raw numbers, got %q", reason)
	}
}

// D4 grandfathering at the equip seam. A character ALREADY past the cap must
// still be able to swap one reserving item for an equally reserving one; a plain
// over-the-cap test would refuse that and force them to strip.
func TestWear_GrandfatheredCharacterCanStillSidegrade(t *testing.T) {
	defer seedMediumSpecies()()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999952: {ItemId: 999952, Name: "old yoke", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.80},
		999953: {ItemId: 999953, Name: "new yoke", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.80},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999952) // 80, already past the 66 cap
	c.Validate()

	_, worn, reason := c.Wear(items.New(999953))
	if !worn {
		t.Fatalf("an equal-for-equal swap must be allowed even past the cap, refused with %q", reason)
	}
	if c.Equipment.Neck.ItemId != 999953 {
		t.Errorf("the swap did not take: neck holds %d", c.Equipment.Neck.ItemId)
	}
}

// The other half of the grandfathering contract, and the reason Worsened exists
// rather than a plain cap test. The same already-over character who may sidegrade
// freely must still be refused a swap that makes the overage BIGGER.
func TestWear_GrandfatheredCharacterIsStillRefusedAnUpgradeInWeight(t *testing.T) {
	defer seedMediumSpecies()()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999957: {ItemId: 999957, Name: "old yoke", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.80},
		999958: {ItemId: 999958, Name: "heavier yoke", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.90},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999957) // 80, already past the 66 cap
	c.Validate()

	returned, worn, reason := c.Wear(items.New(999958))
	if worn {
		t.Fatalf("a swap that deepens the breach must be refused, not worn")
	}
	if len(returned) != 0 {
		t.Errorf("a refused swap must displace nothing, got %d items back", len(returned))
	}
	if c.Equipment.Neck.ItemId != 999957 {
		t.Errorf("a refused swap must leave the original in place, neck holds %d", c.Equipment.Neck.ItemId)
	}
	if !strings.Contains(strings.ToLower(reason), "reserve") {
		t.Errorf("the refusal must name reservation as the cause, got %q", reason)
	}
}

// An equip that REDUCES reservation must always be allowed, however far over the
// character already is. Nothing here may ever force or block a removal.
func TestWear_ADowngradeIsAlwaysAllowed(t *testing.T) {
	defer seedMediumSpecies()()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999954: {ItemId: 999954, Name: "heavy yoke", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.80},
		999955: {ItemId: 999955, Name: "light scarf", Type: items.Neck, Subtype: items.Wearable},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999954)
	c.Validate()

	if _, worn, reason := c.Wear(items.New(999955)); !worn {
		t.Fatalf("swapping to unreserved gear must always be allowed, refused with %q", reason)
	}
}

// An ordinary equip by an unreserved character must be completely unaffected.
// The overwhelming majority of equips are this case and not one number may move.
func TestWear_OrdinaryEquipIsUnchanged(t *testing.T) {
	defer seedMediumSpecies()()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999956: {ItemId: 999956, Name: "plain cap", Type: items.Head, Subtype: items.Wearable},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Validate()

	if _, worn, reason := c.Wear(items.New(999956)); !worn {
		t.Fatalf("an ordinary equip must succeed, refused with %q", reason)
	}
	if c.Equipment.Head.ItemId != 999956 {
		t.Errorf("the cap is not on the head")
	}
}
