package items

import "testing"

// seedSharedTagItems registers the four real bottles, which all share
// component_tag "bottle" and differ in material tier and aging speed.
func seedSharedTagItems(t *testing.T) func() {
	t.Helper()
	return SeedItemsForTest(map[int]*ItemSpec{
		40043: {ItemId: 40043, Name: "clay flask", ComponentTag: "bottle", MaterialTier: 1, BottleAgingMultiplier: 3.0},
		40006: {ItemId: 40006, Name: "glass vial", ComponentTag: "bottle", MaterialTier: 1, BottleAgingMultiplier: 1.0},
		40044: {ItemId: 40044, Name: "sealed phial", ComponentTag: "bottle", MaterialTier: 3, BottleAgingMultiplier: 0.5},
		40045: {ItemId: 40045, Name: "crystalline decanter", ComponentTag: "bottle", MaterialTier: 4, BottleAgingMultiplier: 0.25},
	})
}

// TestFindSpecByComponentTagIsDeterministic pins the fix for a BUG FOUND IN
// PLAYTEST on 2026-08-29.
//
// 🔴 This function ranged a Go map and returned the FIRST match, so with four
// items sharing component_tag "bottle" it had no defined answer. Its callers
// include actions.storeRecovered, which CREATES the item a player receives from
// salvage — so salvaging a potion returned an arbitrary bottle, and a player
// could farm Crystalline Decanters (the most valuable bottle in the game, 0.25x
// aging) out of Clay Flasks by crafting and salvaging in a loop.
//
// Determinism is asserted over many calls because a map range that happens to
// agree twice proves nothing; Go randomises the start offset per range.
func TestFindSpecByComponentTagIsDeterministic(t *testing.T) {
	defer seedSharedTagItems(t)()

	first := FindSpecByComponentTag("bottle")
	if first == nil {
		t.Fatal("no spec resolved for a tag four items carry")
	}
	for i := 0; i < 500; i++ {
		got := FindSpecByComponentTag("bottle")
		if got == nil || got.ItemId != first.ItemId {
			t.Fatalf("resolution flipped between identical calls: %d then %v", first.ItemId, got)
		}
	}
}

// TestFindSpecByComponentTagCannotUpgradeMaterials pins the property that
// closes the exploit: the resolved item is always the CHEAPEST carrying the
// tag, so a tag can never be redeemed for something better than the commonest
// form of that material.
//
// Tie-break is lowest ItemId, which is arbitrary but total — within one tier
// the items are equivalent for craft difficulty.
func TestFindSpecByComponentTagCannotUpgradeMaterials(t *testing.T) {
	defer seedSharedTagItems(t)()

	got := FindSpecByComponentTag("bottle")
	if got == nil {
		t.Fatal("nil spec")
	}
	if got.MaterialTier != 1 {
		t.Fatalf("resolved tier %d (item %d); want tier 1. Anything dearer lets a "+
			"craft-then-salvage loop farm better materials than it consumes.",
			got.MaterialTier, got.ItemId)
	}
	if got.ItemId != 40006 {
		t.Errorf("resolved item %d; want 40006 (lowest id among the tier-1 items)", got.ItemId)
	}
}

// TestFindSpecByComponentTagUnknownTag pins the nil contract.
func TestFindSpecByComponentTagUnknownTag(t *testing.T) {
	defer seedSharedTagItems(t)()
	if got := FindSpecByComponentTag("not-a-tag"); got != nil {
		t.Fatalf("resolved %v for an unknown tag; want nil", got)
	}
}
