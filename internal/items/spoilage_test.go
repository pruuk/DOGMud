package items

import "testing"

func TestSpoilage(t *testing.T) {
	cleanup := SeedItemsForTest(map[int]*ItemSpec{
		1: {ItemId: 1, Name: "Raw Meat", SpoilAfter: "2 days"},
		2: {ItemId: 2, Name: "Bone"},
	})
	defer cleanup()

	shopMeat := Item{ItemId: 1}
	if shopMeat.Spoils() || shopMeat.IsSpoiled(1<<40) {
		t.Error("an unstamped (shop or legacy) item never spoils")
	}
	bone := Item{ItemId: 2, CraftedRound: 100}
	if bone.Spoils() || bone.Freshness(1<<40) != 1 {
		t.Error("an item with no spoil_after never spoils")
	}

	meat := Item{ItemId: 1, CraftedRound: 1000}
	end := meat.SpoilRound()
	if end <= 1000 {
		t.Fatalf("spoil round %d should be after harvest", end)
	}
	if meat.IsSpoiled(end-1) || !meat.IsSpoiled(end) {
		t.Error("spoils exactly at the spoil round")
	}
	mid := 1000 + (end-1000)/2
	if f := meat.Freshness(mid); f < 0.45 || f > 0.55 {
		t.Errorf("halfway freshness = %v, want about 0.5", f)
	}
	if m := meat.FreshnessValueMultiplier(1000); m != 1.0 {
		t.Errorf("fresh sells at full price, got %v", m)
	}
	if m := meat.FreshnessValueMultiplier(end); m != 0.5 {
		t.Errorf("at spoiling the multiplier bottoms at 0.5, got %v", m)
	}

	other := Item{ItemId: 1, CraftedRound: 2000}
	if SameStack(meat, other) {
		t.Error("cuts harvested at different times keep separate clocks")
	}
}
