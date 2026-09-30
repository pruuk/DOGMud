package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/shops"
)

// sevenMerchants are the merchants whose descriptions and dialogue said they
// sold things while they had no shop (merchants slice, 2026-09-30).
var sevenMerchants = []int{
	102,  // Market Merchant
	88,   // Traveling Merchant
	273,  // Whisper
	278,  // Haral
	348,  // Miller Bram
	9309, // Fishmonger
	9509, // Aldith the Bookseller
}

// TestSevenMerchantsAreRealShops pins the merchants slice: each of the seven
// has a shop block of real, salable items, carries the shop housekeeping the
// other shops carry, and the whole shipped mob set still passes the boot-time
// shop validation (shops.ValidateShopMobTags, which panics a cold boot).
//
// Entries are bare item ids: the living economy seeds restock and max stock
// from the item (mobs.RegisterMobShop), so an authored quantity or price would
// be read only by the legacy catalog and would mislead.
//
// Whisper (273) must not become a fence: the owner ruled against it, and a
// fence group would also put Whisper under TestEveryTownHasAFenceNearby.
func TestSevenMerchantsAreRealShops(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	t.Cleanup(items.SeedItemsForTest(nil))
	items.LoadDataFiles()

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataPath)
	if err != nil {
		t.Fatalf("loading mob templates from %s: %v", dataPath, err)
	}

	for _, id := range sevenMerchants {
		m, ok := templates[id]
		if !ok {
			t.Errorf("mob %d is missing from the shipped mob files", id)
			continue
		}
		name := m.Character.Name
		if !m.HasShop() {
			t.Errorf("mob %d %s has no shop block", id, name)
			continue
		}
		if !shops.IsValidCraftSupport(m.ShopCraftSupport) {
			t.Errorf("mob %d %s craft_support %q is not one of %v", id, name, m.ShopCraftSupport, shops.ValidCraftSupports)
		}
		if m.BehaviorArchetype != "noncombat_shopkeeper" {
			t.Errorf("mob %d %s behavior_archetype is %q, want noncombat_shopkeeper", id, name, m.BehaviorArchetype)
		}
		if m.MaxWander != 0 {
			t.Errorf("mob %d %s maxwander is %d, want 0: a wandering keeper leaves the shop empty", id, name, m.MaxWander)
		}
		if !m.NonCombatant {
			t.Errorf("mob %d %s is not non_combatant, as every shop is", id, name)
		}
		if m.IsFence() {
			t.Errorf("mob %d %s is a fence: the slice must not add a fence group", id, name)
		}
		seen := map[int]bool{}
		for _, si := range m.Character.Shop {
			if si.ItemId <= 0 {
				t.Errorf("mob %d %s has a shop entry with no itemid: %+v", id, name, si)
				continue
			}
			if seen[si.ItemId] {
				t.Errorf("mob %d %s lists item %d twice", id, name, si.ItemId)
			}
			seen[si.ItemId] = true
			spec := items.GetItemSpec(si.ItemId)
			if spec == nil {
				t.Errorf("mob %d %s sells item %d, which does not exist", id, name, si.ItemId)
				continue
			}
			if spec.NotSalable {
				t.Errorf("mob %d %s sells item %d %s, which is not_salable", id, name, si.ItemId, spec.Name)
			}
			if si.Quantity != 0 || si.QuantityMax != 0 || si.Price != 0 {
				t.Errorf("mob %d %s item %d authors quantity, quantitymax or price: the economy sets these, use a bare itemid", id, name, si.ItemId)
			}
		}
	}

	all := make([]shops.ShopBearingMob, 0, len(templates))
	for _, m := range templates {
		all = append(all, m)
	}
	if err := shops.ValidateShopMobTags(all); err != nil {
		t.Errorf("shipped shop validation fails:\n%v", err)
	}
}
