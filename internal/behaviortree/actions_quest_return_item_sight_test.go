package behaviortree

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #454: the quest NPC's "hands back" line printed its name raw. A player who
// gave to a shape in the dark never learned that name, so the line hides it at
// the player's sight.
func TestReturnItem_HandsBackLineHidesTheNameInTheDark(t *testing.T) {
	out := returnItemTold(t, false)
	if strings.Contains(out, "test guard") {
		t.Fatalf("a player who sees nothing read the NPC's name: %q", out)
	}
	if !strings.Contains(out, "hands back the brass token") {
		t.Fatalf("the hands-back line must still be sent: %q", out)
	}
}

// The lit control (#454 review F8): a player who sees the NPC reads its name,
// so the dark test cannot pass by the name always being hidden.
func TestReturnItem_HandsBackLineNamesTheNPCInTheLight(t *testing.T) {
	out := returnItemTold(t, true)
	if !strings.Contains(out, "test guard hands back the brass token") {
		t.Fatalf("a player who sees the NPC must read its name: %q", out)
	}
}

// returnItemTold runs actReturnItem in the fixture room, lit or pitch dark,
// and returns what the giver was told.
func returnItemTold(t *testing.T, lit bool) string {
	t.Helper()
	cleanup := seedReturnItemFixture(t)
	t.Cleanup(cleanup)
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	room := rooms.LoadRoom(riRoomId)
	room.Biome = "cave"
	room.Lamp = nil
	if lit {
		room.Lamp = rooms.LampPtr(90)
	} else if room.LightLevel() != 0 {
		t.Fatalf("fixture room must be pitch dark, got light %d", room.LightLevel())
	}

	mob := mobs.GetInstance(riInstanceId)
	given := items.New(riItemId)
	if !mob.Character.StoreItem(given) {
		t.Fatal("setup: mob could not store the given item")
	}
	user := users.GetByUserId(riUserId)
	events.DrainQueuedMessagesForTest(user.UserId)

	ctx := &EvalContext{
		InstanceId: riInstanceId, MobId: riTemplateId, RoomId: riRoomId, MobName: "test guard",
		Event: EventContext{EventType: "player_give", UserId: riUserId, ItemId: riItemId, ItemUUID: given.UUID, RoomId: riRoomId},
	}
	if res := actReturnItem(nil, ctx); res != Success {
		t.Fatalf("actReturnItem: want Success, got %v", res)
	}
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
}
