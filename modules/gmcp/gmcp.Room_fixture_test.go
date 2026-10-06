package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// R9, sibling path: the web client's room contents list is the GMCP twin of
// "On the Ground", so a fixture (part of the room) is left out of it too.
func TestRoomContentsItemsLeaveOutFixtures(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999993: {ItemId: 999993, Name: "Arch Lantern", Type: items.Object, Fixture: items.FixtureLight},
		999994: {ItemId: 999994, Name: "Grey Pebble", Type: items.Object},
	}))
	room := &rooms.Room{RoomId: 9993, Zone: "probe",
		Items: []items.Item{items.New(999993), items.New(999994)}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9993: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(1, "viewer", "Viewer", 0)
	u.Character.RoomId = 9993

	g := &GMCPRoomModule{}
	data, _ := g.GetRoomNode(u, `Room.Info.Contents.Items`)
	got, ok := data.([]GMCPRoomModule_Payload_Contents_Item)
	if !ok {
		t.Fatalf("payload is %T", data)
	}
	if len(got) != 1 || got[0].Name != "Grey Pebble" {
		t.Errorf("contents items = %+v, want only the pebble", got)
	}
}
