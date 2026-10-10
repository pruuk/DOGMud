package items_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// loadShippedWorld loads the shipped conditions and items and restores the
// registries this binary held before, so the shipped set does not replace
// the specs other tests registered (#440: the stacking tests' init specs
// vanished when this ran first under -shuffle). Each Seed call snapshots the
// live registry; LoadDataFiles then swaps in its own map.
func loadShippedWorld(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(items.SeedAttackMessagesForTest(nil))
	t.Cleanup(items.SeedDefenseMessagesForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
}

// The 5a ladder, read from the shipped world: every carried light has its
// strength, lives in the light slot, and is secret so a light shows at most
// once in the conditions list (owner, 2026-09-26).
func TestShippedLightItemsMatchTheLadder(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	// Condition 0 derives its TriggerCount from this at Validate time and
	// refuses 0; a test binary never reads config.yaml.
	cfg.Network.LogoutRounds = 3
	configs.SetConfigForTest(t, cfg)
	loadShippedWorld(t)

	cases := []struct {
		itemId     int
		strength   float64
		adjustable bool
	}{
		{40077, 38, false}, // Tallow Candle
		{40038, 52, false}, // Oil Lantern
		{20096, 56, false}, // Torch
		{20097, 54, true},  // Hooded Lantern
		{20099, 46, false}, // Sunstone (lighting 5e)
	}
	for _, c := range cases {
		spec := items.GetItemSpec(c.itemId)
		if spec == nil {
			t.Fatalf("item %d is not shipped", c.itemId)
		}
		if spec.Type != items.Light || spec.Subtype != items.Wearable {
			t.Errorf("item %d is %s/%s, want light/wearable", c.itemId, spec.Type, spec.Subtype)
		}
		if len(spec.WornConditionIds) != 1 {
			t.Fatalf("item %d grants %d conditions, want 1", c.itemId, len(spec.WornConditionIds))
		}
		cond := conditions.GetConditionSpec(spec.WornConditionIds[0])
		if cond == nil || !cond.Secret {
			t.Errorf("item %d's light condition must exist and be secret", c.itemId)
			continue
		}
		if v := cond.Effects[conditions.EffectLightStrength]; v.UsesMagnitude || v.Literal != c.strength {
			t.Errorf("item %d shines at %+v, want %v", c.itemId, v, c.strength)
		}
		adjustable := false
		for _, f := range cond.Flags {
			adjustable = adjustable || f == conditions.Adjustable
		}
		if adjustable != c.adjustable {
			t.Errorf("item %d adjustable = %v, want %v", c.itemId, adjustable, c.adjustable)
		}
	}
	if spec := items.GetItemSpec(20097); spec == nil || spec.Nouns["hood"] == "" {
		t.Error("the hooded lantern must carry a hood noun")
	}

	// Lighting plan 5d: the Umbral Lantern is the ladder's one darkness, a
	// light-slot item whose one secret condition takes 50 away, adjustable,
	// with no hood and never a light.
	umbral := items.GetItemSpec(20098)
	if umbral == nil {
		t.Fatal("item 20098 (Umbral Lantern) is not shipped")
	}
	if umbral.Type != items.Light || umbral.Subtype != items.Wearable || len(umbral.WornConditionIds) != 1 {
		t.Fatalf("Umbral Lantern is %s/%s with %d conditions, want light/wearable with 1", umbral.Type, umbral.Subtype, len(umbral.WornConditionIds))
	}
	dark := conditions.GetConditionSpec(umbral.WornConditionIds[0])
	if dark == nil || !dark.Secret || dark.IsLightSource() || !dark.IsDarknessSource() {
		t.Fatalf("the Umbral Lantern's condition must exist, be secret, and be a darkness and not a light: %+v", dark)
	}
	if v := dark.Effects[conditions.EffectDarknessStrength]; v.UsesMagnitude || v.Literal != 50 {
		t.Errorf("Umbral Lantern darkens at %+v, want 50", v)
	}
	adjustable := false
	for _, f := range dark.Flags {
		adjustable = adjustable || f == conditions.Adjustable
	}
	if !adjustable {
		t.Error("the Umbral Lantern's darkness must be adjustable")
	}
	if len(umbral.Nouns) != 0 {
		t.Errorf("the Umbral Lantern carries nouns %v, want none (it has no hood)", umbral.Nouns)
	}
	// Boot refuses an economy item with no vendor_categories
	// (items.ValidateVendorCategories); the list says who buys it, not who
	// stocks it, so the boss drop still sells.
	if len(umbral.VendorCategories) != 1 || umbral.VendorCategories[0] != "blacksmithing" {
		t.Errorf("Umbral Lantern vendor_categories = %v, want [blacksmithing], the hooded lantern's", umbral.VendorCategories)
	}
}

// Lighting 5e: the trees and fixtures slice 1 ships. The Oil Lantern carries
// the keeper lantern tree (owner ruling R3); the sunstone is worth 40 (between
// the hooded lantern's 20 and the Umbral Lantern's 60) and sells to an
// enchanter; the two fixtures are fixed lights nobody can sell.
func TestShippedItemBehaviours(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	cfg.Network.LogoutRounds = 3
	configs.SetConfigForTest(t, cfg)
	loadShippedWorld(t)

	for _, c := range []struct {
		itemId   int
		behavior string
		fixture  string
	}{
		{40038, "keeper_lantern", ""},
		{20099, "sunstone", ""},
		{55, "dusk_to_dawn", items.FixtureLight},
		{56, "rift_pulse", items.FixtureLight},
	} {
		spec := items.GetItemSpec(c.itemId)
		if spec == nil {
			t.Fatalf("item %d is not shipped", c.itemId)
		}
		if spec.Behavior != c.behavior || spec.Fixture != c.fixture {
			t.Errorf("item %d: behavior %q fixture %q, want %q and %q", c.itemId, spec.Behavior, spec.Fixture, c.behavior, c.fixture)
		}
		if c.fixture != "" && !spec.NotSalable {
			t.Errorf("fixture %d must be not_salable: it is never loot", c.itemId)
		}
	}
	sun := items.GetItemSpec(20099)
	if sun.Value != 40 || len(sun.VendorCategories) != 1 || sun.VendorCategories[0] != "enchanting" {
		t.Errorf("sunstone value %d, vendor_categories %v; want 40 and [enchanting]", sun.Value, sun.VendorCategories)
	}
}
