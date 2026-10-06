package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
)

const (
	floorTreedItem = 999960
	floorPlainItem = 999961
)

// seedFloorIndex seeds one treed and one plain item and an empty index, and
// records every room the index reports entering.
func seedFloorIndex(t *testing.T) *[]int {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		floorTreedItem: {ItemId: floorTreedItem, Name: "test arch lantern", Type: items.Object,
			Fixture: items.FixtureLight, Behavior: "dusk_to_dawn"},
		floorPlainItem: {ItemId: floorPlainItem, Name: "test pebble", Type: items.Object},
	}))
	t.Cleanup(items.ResetHolderIndexForTest())
	t.Cleanup(itemlight.ResetForTest())
	var entered []int
	prev := items.OnRoomHolderIndexed
	items.OnRoomHolderIndexed = func(roomId int) { entered = append(entered, roomId) }
	t.Cleanup(func() { items.OnRoomHolderIndexed = prev })
	return &entered
}

// Rule 5: a room joins the index when a treed item lands on its floor, by
// AddItem or by Prepare's spawn append; a plain item or a stash does not
// index it.
func TestATreedFloorItemIndexesItsRoom(t *testing.T) {
	entered := seedFloorIndex(t)

	plain := &Room{RoomId: 7801}
	plain.AddItem(items.New(floorPlainItem), false)
	stashed := &Room{RoomId: 7802}
	stashed.AddItem(items.New(floorTreedItem), true)
	added := &Room{RoomId: 7803}
	added.AddItem(items.New(floorTreedItem), false)
	spawned := &Room{RoomId: 7804, SpawnInfo: []SpawnInfo{{ItemId: floorTreedItem}}}
	spawned.Prepare(false)

	if got := items.RoomHolders(); len(got) != 2 || got[0] != 7803 || got[1] != 7804 {
		t.Errorf("RoomHolders = %v, want [7803 7804]", got)
	}
	if len(*entered) != 2 {
		t.Errorf("OnRoomHolderIndexed saw %v, want the two rooms once each", *entered)
	}
}

// A room loading into memory with a treed item already on its floor (kept
// by its instance file) joins the index, and IndexTreedFloors catches a room
// loaded before item specs existed.
func TestALoadedRoomHoldingATreedItemIsIndexed(t *testing.T) {
	seedFloorIndex(t)
	t.Cleanup(SeedRoomsForTest(map[int]*Room{}, map[string]*ZoneConfig{}))

	loaded := &Room{RoomId: 7811, Zone: "probe", Items: []items.Item{items.New(floorTreedItem)}}
	if err := addRoomToMemory(loaded); err != nil {
		t.Fatal(err)
	}
	if got := items.RoomHolders(); len(got) != 1 || got[0] != 7811 {
		t.Fatalf("after addRoomToMemory RoomHolders = %v, want [7811]", got)
	}

	early := &Room{RoomId: 7812, Zone: "probe", Items: []items.Item{items.New(floorTreedItem)}}
	roomManager.rooms[early.RoomId] = early // in memory, never indexed
	IndexTreedFloors()
	if got := items.RoomHolders(); len(got) != 2 || got[1] != 7812 {
		t.Errorf("after IndexTreedFloors RoomHolders = %v, want [7811 7812]", got)
	}
}

// Rule 10: an item taken off the floor stops lighting the room.
func TestRemoveItemDropsAFixturesOutput(t *testing.T) {
	seedFloorIndex(t)
	r := &Room{RoomId: 7821}
	lamp := items.New(floorTreedItem)
	r.AddItem(lamp, false)
	itemlight.Set(r.RoomId, lamp.UUID, itemlight.Light, 52)
	r.RemoveItem(lamp, false)
	if _, ok := itemlight.Get(r.RoomId, lamp.UUID); ok {
		t.Error("the removed fixture still has an output")
	}
}
