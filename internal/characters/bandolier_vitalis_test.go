package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// #794, owner ruling 2026-11-06: the Vitalis Bandolier holds its potions at
// peak strength, refuses anything whose held effect would not help, and a
// potion taken out of it rots at once.

const (
	vtVitalisId   = 999980
	vtPlainBandId = 999981
	vtSalveId     = 999982 // a helpful potion that ages
	vtRedId       = 999983 // a helpful potion that never ages
	vtPurgeId     = items.YsoldesPurgeItemId
	vtFlaskId     = 999984 // a throwable
	vtWaferId     = 999985 // a potion with no condition to hold
	vtNow         = uint64(5_000_000)
)

var vtAging = items.AgingThresholds{FermentRounds: 1000, PeakRounds: 5000, DecayRounds: 15000, SpoilRounds: 25000}

func vtSeed(t *testing.T) {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		vtVitalisId: {ItemId: vtVitalisId, Name: "Vitalis Bandolier", Type: items.Belt, Subtype: items.Wearable,
			IsBandolier: true, BandolierCapacity: 4, AmbientPotions: true, PreservesContents: true},
		vtPlainBandId: {ItemId: vtPlainBandId, Name: "leather bandolier", Type: items.Belt, Subtype: items.Wearable,
			IsBandolier: true, BandolierCapacity: 12},
		vtSalveId: {ItemId: vtSalveId, Name: "healing salve", Type: items.Potion, Subtype: items.Drinkable,
			ConditionIds: []int{54}, Weight: 0.1, Aging: vtAging},
		vtRedId: {ItemId: vtRedId, Name: "small red potion", Type: items.Potion, Subtype: items.Drinkable,
			ConditionIds: []int{5}, Weight: 0.1},
		vtPurgeId: {ItemId: vtPurgeId, Name: "Ysolde's Purge", Type: items.Potion, Subtype: items.Drinkable,
			ConditionIds: []int{93}, Weight: 0.1},
		vtFlaskId: {ItemId: vtFlaskId, Name: "toxic flask", Type: items.Object, Subtype: items.Throwable,
			ConditionIds: []int{78}, Weight: 0.1},
		vtWaferId: {ItemId: vtWaferId, Name: "bloom wafer", Type: items.Potion, Subtype: items.Drinkable, Weight: 0.1},
	}))
	prev := util.GetRoundCount()
	util.SetRoundCountForTest(vtNow)
	t.Cleanup(func() { util.SetRoundCountForTest(prev) })
}

func vtWearer(t *testing.T, beltId int) *Character {
	t.Helper()
	c := New()
	c.Stats.Strength.ValueAdj = 100
	c.Equipment.Belt = items.New(beltId)
	return c
}

func vtIn(list []items.Item, itemId int) bool {
	for _, it := range list {
		if it.ItemId == itemId {
			return true
		}
	}
	return false
}

func TestVitalisRefusesWhatWouldNotHelp(t *testing.T) {
	vtSeed(t)
	c := vtWearer(t, vtVitalisId)

	spoiled := items.New(vtSalveId)
	spoiled.CraftedRound = vtNow - 30000 // past its spoil point
	for _, itm := range []items.Item{items.New(vtPurgeId), items.New(vtFlaskId), items.New(vtWaferId), spoiled, items.New(vtSalveId)} {
		if !c.StoreItem(itm) {
			t.Fatalf("StoreItem refused %s outright", itm.Name())
		}
	}
	if !vtIn(c.PotionItems, vtSalveId) || len(c.PotionItems) != 1 {
		t.Fatalf("only the fresh salve belongs in the Vitalis, got %d slotted", len(c.PotionItems))
	}
	for _, id := range []int{vtPurgeId, vtFlaskId, vtWaferId} {
		if !vtIn(c.Items, id) {
			t.Errorf("item %d should be routed to the backpack", id)
		}
	}

	// An ordinary bandolier still holds a throwable, so throw finds its ammunition.
	plain := vtWearer(t, vtPlainBandId)
	plain.StoreItem(items.New(vtFlaskId))
	if !vtIn(plain.PotionItems, vtFlaskId) {
		t.Fatal("an ordinary bandolier must still take a throwable")
	}

	// sort names what the Vitalis refused and leaves it in the backpack.
	s := vtWearer(t, vtVitalisId)
	s.Items = []items.Item{items.New(vtFlaskId), items.New(vtSalveId)}
	moved, refused := s.SortPotionItems()
	if moved != 1 || len(refused) != 1 || refused[0].ItemId != vtFlaskId {
		t.Fatalf("sort should move the salve and refuse the flask, moved %d refused %v", moved, refused)
	}
}

func TestVitalisHoldsEveryPotionAtPeak(t *testing.T) {
	vtSeed(t)
	c := vtWearer(t, vtVitalisId)
	fresh := items.New(vtSalveId)
	fresh.CraftedRound = vtNow - 10 // fresh when slotted
	c.StoreItem(fresh)
	c.StoreItem(items.New(vtRedId)) // shop-bought, never crafted
	for _, p := range c.PotionItems {
		got := c.PotionAging(p, vtNow+100000)
		if got.Phase != items.PhasePeak || got.Potency != items.PeakPotency {
			t.Errorf("%s should read at peak in the Vitalis, got phase %v potency %v", p.Name(), got.Phase, got.Potency)
		}
	}
}

func TestPotionTakenOutOfTheVitalisRots(t *testing.T) {
	vtSeed(t)
	c := vtWearer(t, vtVitalisId)
	c.StoreItem(items.New(vtSalveId))
	c.StoreItem(items.New(vtRedId))

	// Any path: find it in the bandolier, then remove it (give, sell, put...).
	for _, name := range []string{"healing salve", "small red potion"} {
		held, ok := c.FindInPotions(name)
		if !ok {
			t.Fatalf("%s should be slotted", name)
		}
		if !c.RotsOnRemoval(held) {
			t.Errorf("%s: RotsOnRemoval should be true while slotted", name)
		}
		c.RemoveItem(held)
		if !held.AgingAt(vtNow).Spoiled() {
			t.Errorf("%s taken out of the Vitalis must read spoiled", name)
		}
		// And it cannot go back in to be brought back to peak.
		c.StoreItem(held)
		if vtIn(c.PotionItems, held.ItemId) {
			t.Errorf("a rotted %s must not slot back into the Vitalis", name)
		}
	}
}

func TestTakingOffOrSwappingTheVitalisRotsItsPotions(t *testing.T) {
	vtSeed(t)

	off := vtWearer(t, vtVitalisId)
	off.StoreItem(items.New(vtSalveId))
	if len(off.BeltChangeRots()) != 1 {
		t.Fatal("BeltChangeRots should name the slotted salve")
	}
	off.RemoveFromBody(off.Equipment.Belt)
	if len(off.Items) != 1 || !off.Items[0].AgingAt(vtNow).Spoiled() {
		t.Fatal("taking the Vitalis off must spill its potions to the backpack, rotted")
	}

	swap := vtWearer(t, vtVitalisId)
	swap.StoreItem(items.New(vtSalveId))
	if _, worn, why := swap.Wear(items.New(vtPlainBandId)); !worn {
		t.Fatalf("wearing the plain bandolier failed: %s", why)
	}
	if len(swap.PotionItems) != 0 || len(swap.Items) != 1 || !swap.Items[0].AgingAt(vtNow).Spoiled() {
		t.Fatalf("swapping the Vitalis out must spill its potions rotted, slotted %d pack %d",
			len(swap.PotionItems), len(swap.Items))
	}

	// A rotted potion slotted into an ordinary bandolier does not come back
	// to peak when a Vitalis goes on over it.
	back := vtWearer(t, vtPlainBandId)
	rotted := items.New(vtSalveId)
	rotted.Preserved = true
	back.PotionItems = []items.Item{rotted}
	if _, worn, why := back.Wear(items.New(vtVitalisId)); !worn {
		t.Fatalf("wearing the Vitalis failed: %s", why)
	}
	if len(back.PotionItems) != 0 {
		t.Fatal("a rotted potion must leave the slots when a Vitalis goes on")
	}
}
