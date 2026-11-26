package ferry

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The tie-up line is when a passenger needs the exit's word, and since owner
// ruling 18 (2026-11-26) that word is plank, not gangplank: typing gangplank
// finds no exit. So the deck's arrival line names the exit to type.
func TestArrival_DeckLineNamesTheExitToType(t *testing.T) {
	const dockId, deckId = 9861, 9862
	dock := &rooms.Room{RoomId: dockId, Zone: "FerryZone"}
	deck := &rooms.Room{RoomId: deckId, Zone: "FerryZone"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{dockId: dock, deckId: deck},
		map[string]*rooms.ZoneConfig{"FerryZone": {Name: "FerryZone", RoomId: dockId,
			RoomIds: map[int]struct{}{dockId: {}, deckId: {}}}}))
	passenger := users.NewTestUser(1, "pas", "Passa", 1001)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: passenger}))
	passenger.Character.RoomId = deckId
	deck.AddPlayer(passenger.UserId)
	events.DrainQueuedMessagesForTest(passenger.UserId)

	emitArrival(Route{Name: "the Test Packet", DeckRoom: deckId,
		Ports: []Port{{DockRoom: dockId}, {DockRoom: 9863}}}, 0)

	got := strings.Join(events.DrainQueuedMessagesForTest(passenger.UserId), "")
	require.Contains(t, got, `Type <ansi fg="command">`+gangplankExitName+`</ansi> to walk ashore.`)
}
