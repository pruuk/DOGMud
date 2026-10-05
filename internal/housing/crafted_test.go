package housing

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	testChestId     = 9301
	testBedFrameId  = 9302
	testWorkbenchId = 9303
)

// seedFurniture adds crafted furniture specs to the test item set.
func seedFurniture(t *testing.T) {
	t.Helper()
	all := map[int]*items.ItemSpec{}
	for id, spec := range items.GetAllItemSpecsMap() {
		all[id] = spec
	}
	all[testChestId] = &items.ItemSpec{ItemId: testChestId, Name: `Wooden Chest`, NameSimple: `chest`, Type: items.Object, Furnishing: items.FurnishingChest}
	all[testBedFrameId] = &items.ItemSpec{ItemId: testBedFrameId, Name: `Bed Frame`, NameSimple: `frame`, Type: items.Object, Furnishing: items.FurnishingBed}
	all[testWorkbenchId] = &items.ItemSpec{ItemId: testWorkbenchId, Name: `Woodworking Bench`, NameSimple: `workbench`, Type: items.Object, Furnishing: items.FurnishingWorkbench}
	t.Cleanup(items.SeedItemsForTest(all))
}

func give(u *users.UserRecord, itemId int) items.Item {
	itm := items.New(itemId)
	u.Character.Items = append(u.Character.Items, itm)
	return itm
}

func TestCraftedFurniture_PlacesLikeADeed(t *testing.T) {
	setup(t)
	prev := stationTypes
	stationTypes = func() []string { return []string{`forge`, `woodworking_bench`} }
	t.Cleanup(func() { stationTypes = prev })
	seedFurniture(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferBed)
	home := w.homeOf(t, u)

	if !IsHousingItem(testChestId) || !IsCraftedFurnishing(testBedFrameId) {
		t.Fatal("crafted furniture counts as a housing item, so only housing spends it")
	}

	UseItem(u, w.room(home), give(u, testChestId), `trunk`, `chest trunk`)
	if _, ok := w.room(home).Containers[`trunk`]; !ok {
		t.Error("the crafted chest should be placed as a container")
	}
	if itemOf(u, testChestId).ItemId != 0 {
		t.Error("the chest is spent when placed")
	}

	UseItem(u, w.room(home), give(u, testBedFrameId), ``, `frame`)
	if !RoomHasBed(home) {
		t.Error("the bed frame should set up a bed")
	}

	UseItem(u, w.room(home), give(u, testWorkbenchId), ``, `workbench`)
	if st := w.room(home).Station; st != `woodworking_bench` {
		t.Errorf("the workbench should install a woodworking bench, room station %q", st)
	}
}

func TestCraftedFurniture_OnlyInYourOwnLodging(t *testing.T) {
	setup(t)
	seedFurniture(t)
	w := newContainerWorld(t)
	owner := w.lodger(t, 1, `Alice`, OfferBed)
	home := w.homeOf(t, owner)
	other := w.player(2, `Bob`, 1000)
	other.Character.RoomId = home

	UseItem(other, w.room(home), give(other, testChestId), `trunk`, `chest trunk`)
	if _, ok := w.room(home).Containers[`trunk`]; ok {
		t.Error("a visitor cannot furnish someone else's lodging")
	}
	if itemOf(other, testChestId).ItemId == 0 {
		t.Error("a refused chest is not spent")
	}
}
