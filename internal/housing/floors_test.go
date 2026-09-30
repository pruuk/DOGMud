package housing

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Floors: what lies on a lodging's floor lives in the house record, so it
// survives a restart even when the room's instance save is gone (the reload
// below builds every room fresh from its template, as after a wipe).

func floorOf(t *testing.T, userId int, roomId int) (HouseFloor, bool) {
	t.Helper()
	h, _ := HouseOf(userId, testBldgId)
	f, _, ok := h.floorOf(roomId)
	return f, ok
}

func TestFloor_DroppedItemsAndGoldSurviveARestartWithoutInstanceSaves(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	r := w.room(home)

	// "drop spoon", "drop 30 gold", "stash ring".
	spoon, ring := trinket(testKeyId), trinket(testVoucherId)
	r.Items = append(r.Items, spoon)
	r.Gold = 30
	r.Stash = append(r.Stash, ring)
	AfterUserCommand(1, home)

	f, ok := floorOf(t, 1, home)
	if !ok || len(f.Items) != 1 || !f.Items[0].Equals(spoon) || f.Gold != 30 || len(f.Stash) != 1 {
		t.Fatalf("recorded floor = %+v, %v", f, ok)
	}
	if !w.saved[1] {
		t.Error("the player was not saved alongside the floor")
	}

	w.reload(t)
	back := w.room(home)
	if len(back.Items) != 1 || back.Items[0].ItemId != spoon.ItemId || back.Items[0].UUID.IsNil() ||
		back.Gold != 30 || len(back.Stash) != 1 || back.Stash[0].ItemId != ring.ItemId {
		t.Fatalf("after restart the floor is items=%+v gold=%d stash=%+v", back.Items, back.Gold, back.Stash)
	}
	if Capture(back) {
		t.Error("a freshly loaded floor differs from its own record")
	}
}

func TestFloor_PickingEverythingUpIsRecordedToo(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	r := w.room(home)
	r.Items = append(r.Items, trinket(testKeyId))
	AfterUserCommand(1, home)

	r.Items = r.Items[:0] // "get spoon"
	AfterUserCommand(1, home)
	if f, ok := floorOf(t, 1, home); !ok || len(f.Items) != 0 {
		t.Fatalf("recorded floor after pickup = %+v, %v", f, ok)
	}

	// An old instance save that still has the spoon must not bring it back.
	stale := unitRoom(home)
	stale.Items = []items.Item{trinket(testKeyId)}
	ApplyOverlay(stale)
	if len(stale.Items) != 0 {
		t.Errorf("a picked-up item came back from an old instance save: %+v", stale.Items)
	}
}

func TestFloor_ExistingFloorIsAdoptedNotLost(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)

	// A lodging from before floors were recorded: its floor is only in the
	// instance save, and the load leaves it there.
	r := unitRoom(home)
	r.Items = []items.Item{trinket(testKeyId)}
	r.Gold = 4
	ApplyOverlay(r)
	if len(r.Items) != 1 || r.Gold != 4 {
		t.Fatalf("an unrecorded floor was cleared on load: %+v", r.Items)
	}
	w.rooms[home] = r

	// The first capture adopts it.
	if !Capture(r) {
		t.Fatal("the existing floor was not adopted")
	}
	w.reload(t)
	if back := w.room(home); len(back.Items) != 1 || back.Gold != 4 {
		t.Errorf("adopted floor after restart = %+v gold %d", back.Items, back.Gold)
	}
}

func TestFloor_AnEmptyUnrecordedFloorWritesNothing(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	w.room(home)
	AfterUserCommand(1, home)
	if w.saved[1] {
		t.Error("an empty floor caused a write")
	}
	if _, ok := floorOf(t, 1, home); ok {
		t.Error("an empty floor was recorded")
	}
}

func TestFloor_DropThenWalkOutIsCaptured(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	r := w.room(home)

	// One command line that ends with the player in the alley.
	r.Items = append(r.Items, trinket(testKeyId))
	u.Character.RoomId = testDoor
	AfterUserCommand(1, home)
	if f, ok := floorOf(t, 1, home); !ok || len(f.Items) != 1 {
		t.Fatalf("the floor of the room left behind was not captured: %+v", f)
	}
}

func TestFloor_EveryLoadedRoomOfTheHouseIsCaptured(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	buyKey(u, OfferExtension)
	UseItem(u, w.room(home), itemOf(u, testDeedId), `north`, `deed north`)
	h, _ := HouseOf(1, testBldgId)
	back := w.room(h.RoomIds[1])

	// Something lands in the next room while the player stands at home.
	back.Items = append(back.Items, trinket(testKeyId))
	AfterUserCommand(1, home)
	if f, ok := floorOf(t, 1, back.RoomId); !ok || len(f.Items) != 1 {
		t.Fatalf("the neighbouring room's floor was not captured: %+v", f)
	}
}

func TestFloor_MobsAndAutosaveCaptureToo(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	r := w.room(home)

	r.Gold = 2
	w.mobAt[900] = home
	AfterMobCommand(900, home) // a companion drops something
	if f, _ := floorOf(t, 1, home); f.Gold != 2 {
		t.Error("a mob's command was not captured")
	}
	r.Gold = 3
	CaptureOnSave(r) // an item landed with no command (a corpse rotting)
	if f, _ := floorOf(t, 1, home); f.Gold != 3 {
		t.Error("the autosave did not capture")
	}
}

func TestFloor_VacantUnitsAndOtherRoomsAreLeftAlone(t *testing.T) {
	setup(t)
	newContainerWorld(t)
	vacant := unitRoom(testUnitC)
	vacant.Items = []items.Item{trinket(testKeyId)}
	ApplyOverlay(vacant)
	if len(vacant.Items) != 1 {
		t.Error("a vacant unit's floor was touched")
	}
	if Capture(vacant) || Capture(&rooms.Room{RoomId: 5615, Gold: 9}) {
		t.Error("captured a room that is in no house")
	}
}

func TestLoad_BadFloorsAreHeld(t *testing.T) {
	setup(t)
	for name, h := range map[string]House{
		`room not in house`: {BuildingId: testBldgId, OwnerUserId: 1, RoomIds: []int{testUnitA}, Floors: []HouseFloor{{RoomId: testUnitC}}},
		`recorded twice`:    {BuildingId: testBldgId, OwnerUserId: 1, RoomIds: []int{testUnitA}, Floors: []HouseFloor{{RoomId: testUnitA}, {RoomId: testUnitA}}},
	} {
		if err := h.checkLinks(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestFloor_AStashOnlyChangeIsCaptured(t *testing.T) {
	setup(t)
	w := newContainerWorld(t)
	u := w.lodger(t, 1, `Alice`, OfferContainer)
	home := w.homeOf(t, u)
	r := w.room(home)
	r.Gold = 1
	AfterUserCommand(1, home)

	r.Stash = append(r.Stash, trinket(testKeyId)) // "stash spoon", nothing else moves
	AfterUserCommand(1, home)
	if f, _ := floorOf(t, 1, home); len(f.Stash) != 1 {
		t.Errorf("a stashed item was not captured: %+v", f)
	}
}
