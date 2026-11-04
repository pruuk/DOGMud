package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// FindInPotionsWhere honours N.item over the bandolier's list, oldest first, so
// 1. and 2. never name one potion twice, and counts within keep() as
// FindInBackpackWhere does.
func TestFindInPotionsWhere_Ordinal(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999971: {ItemId: 999971, Name: "healing salve", Type: items.Potion},
		999972: {ItemId: 999972, Name: "greater healing salve", Type: items.Potion},
	})()

	c := New()
	young, plain, old := items.New(999971), items.New(999972), items.New(999971)
	young.CraftedRound, plain.CraftedRound, old.CraftedRound = 9, 1, 5
	c.PotionItems = []items.Item{young, plain, old}

	named := func(name string, keep func(items.Item) bool) items.Item {
		t.Helper()
		got, ok := c.FindInPotionsWhere(name, keep)
		if !ok {
			t.Fatalf("%q found nothing", name)
		}
		return got
	}
	if got := named("healing salve", nil); got.UUID != plain.UUID {
		t.Errorf("no ordinal: want the oldest match, got %s", got.Name())
	}
	onlySalve := func(i items.Item) bool { return i.ItemId == 999971 }
	if got := named("1.healing salve", onlySalve); got.UUID != old.UUID {
		t.Errorf("1. within keep: want the older salve")
	}
	if got := named("2.healing salve", onlySalve); got.UUID != young.UUID {
		t.Errorf("2. within keep: want the younger salve")
	}
	if got := named("healing salve#2", onlySalve); got.UUID != young.UUID {
		t.Errorf("#2 within keep: want the younger salve")
	}
	if _, ok := c.FindInPotionsWhere("3.healing salve", onlySalve); ok {
		t.Error("3. within keep: only two salves pass keep")
	}
}

// TestStoreItem_AmbientBandolier_OnePerType proves an ambient bandolier (Vitalis)
// accepts one of each potion type but routes a duplicate type to the backpack —
// the anti-immortality cap. A distinct potion type still slots normally.
func TestStoreItem_AmbientBandolier_OnePerType(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999970: {ItemId: 999970, Name: "Vitalis Bandolier", Type: items.Belt,
			IsBandolier: true, BandolierCapacity: 4, AmbientPotions: true},
		999971: {ItemId: 999971, Name: "healing salve", Type: items.Potion, ConditionIds: []int{54}, Weight: 0.1},
		999972: {ItemId: 999972, Name: "elixir", Type: items.Potion, ConditionIds: []int{55}, Weight: 0.1},
	})()

	c := New()
	c.Stats.Strength.ValueAdj = 100 // ample carry capacity
	c.Equipment.Belt = items.New(999970)

	// First salve routes into the bandolier.
	c.StoreItem(items.New(999971))
	if len(c.PotionItems) != 1 {
		t.Fatalf("first salve should slot in the bandolier; PotionItems=%d", len(c.PotionItems))
	}

	// A DUPLICATE salve must NOT slot — it falls through to the backpack.
	c.StoreItem(items.New(999971))
	if len(c.PotionItems) != 1 {
		t.Fatalf("duplicate salve must not enter the ambient bandolier; PotionItems=%d", len(c.PotionItems))
	}
	if len(c.Items) != 1 {
		t.Fatalf("duplicate salve should land in the backpack; Items=%d", len(c.Items))
	}

	// A DIFFERENT potion type still slots.
	c.StoreItem(items.New(999972))
	if len(c.PotionItems) != 2 {
		t.Fatalf("a distinct potion type should slot; PotionItems=%d", len(c.PotionItems))
	}
}

// TestStoreItem_NonAmbientBandolier_AllowsDuplicates proves an ordinary storage
// bandolier (no ambient ticking) still accepts duplicate potion types — the cap
// only exists to stop passive stacking, which non-ambient belts don't do.
func TestStoreItem_NonAmbientBandolier_AllowsDuplicates(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999973: {ItemId: 999973, Name: "plain bandolier", Type: items.Belt,
			IsBandolier: true, BandolierCapacity: 4, AmbientPotions: false},
		999974: {ItemId: 999974, Name: "healing salve", Type: items.Potion, ConditionIds: []int{54}, Weight: 0.1},
	})()

	c := New()
	c.Stats.Strength.ValueAdj = 100
	c.Equipment.Belt = items.New(999973)

	c.StoreItem(items.New(999974))
	c.StoreItem(items.New(999974))
	if len(c.PotionItems) != 2 {
		t.Fatalf("a non-ambient bandolier should allow duplicate potion types; PotionItems=%d", len(c.PotionItems))
	}
}
