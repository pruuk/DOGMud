package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// watchedViewer is exitsFixture with the viewer on a GMCP connection and no
// record of a Room.Info sent to them.
func watchedViewer(t *testing.T, lamp int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	viewer, room := exitsFixture(t, lamp)
	gmcpModule.cache.Add(viewer.ConnectionId(), GMCPSettings{GMCPAccepted: true, HelloReceived: true})
	t.Cleanup(func() { gmcpModule.cache.Remove(viewer.ConnectionId()) })
	forgetSentRoomExits(viewer.UserId)
	t.Cleanup(func() { forgetSentRoomExits(viewer.UserId) })
	events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`)
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	t.Cleanup(func() {
		events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`)
		events.DrainQueuedByTypeForTest(`GMCPOut`)
	})
	return viewer, room
}

// sendRoomInfo sends viewer a Room.Info as the queue would.
func sendRoomInfo(viewer *users.UserRecord) {
	(&GMCPRoomModule{}).buildAndSendGMCPPayload(GMCPRoomUpdate{UserId: viewer.UserId, Identifier: `Room.Info`})
	events.DrainQueuedByTypeForTest(`GMCPOut`)
}

// roomInfoUpdates drains the queued Room.Info requests and counts userId's.
func roomInfoUpdates(userId int) int {
	n := 0
	for _, e := range events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`) {
		if u, ok := e.(GMCPRoomUpdate); ok && u.UserId == userId && u.Identifier == `Room.Info` {
			n++
		}
	}
	return n
}

// "Kept current": the watcher re-sends Room.Info on a real change (a door
// unlocked, relocking on its timer, a gangplank coming and going, a secret
// exit found, a light going out) and only then.
func TestRoomExitsWatch_ResendsOnlyOnARealChange(t *testing.T) {
	start := pinRelockTiming(t)
	viewer, room := watchedViewer(t, 60)
	north := room.Exits["north"]
	north.Lock = gamelock.Lock{Difficulty: 5}
	room.Exits["north"] = north
	g := &GMCPRoomModule{}

	sendRoomInfo(viewer)
	g.roomExitsWatchHandler(events.NewRound{})
	require.Zero(t, roomInfoUpdates(viewer.UserId), "an unchanged round sends nothing")

	steps := []struct {
		name   string
		change func()
	}{
		{"a door unlocked", func() { room.SetExitLock("north", false) }},
		{"the door relocked on its timer", func() {
			util.SetRoundCountForTest(gametime.GetDate(start).AddPeriod(gamelock.DefaultRelockTime))
		}},
		{"a gangplank came", func() { room.AddTemporaryExit("gangplank", exit.TemporaryRoomExit{RoomId: 9721}) }},
		{"the gangplank went", func() { room.RemoveTemporaryExit(room.ExitsTemp["gangplank"]) }},
		{"a secret exit found", func() { viewer.Character.LearnSecretExit(9720, "west") }},
		{"the light went out", func() { room.Lamp = nil }},
	}
	for _, s := range steps {
		s.change()
		g.roomExitsWatchHandler(events.NewRound{})
		require.Equal(t, 1, roomInfoUpdates(viewer.UserId), s.name)
		sendRoomInfo(viewer)
		g.roomExitsWatchHandler(events.NewRound{})
		require.Zero(t, roomInfoUpdates(viewer.UserId), "%s: once sent, not repeated", s.name)
	}
	require.Zero(t, roomInfoUpdates(9723), "Kesh has no GMCP connection and is never sent one")
}

// A dark mark sends Room.Info at once, not at the next round.
func TestRoomExitsChanged_SendsRoomInfoAtOnce(t *testing.T) {
	viewer, _ := watchedViewer(t, 0)
	(&GMCPRoomModule{}).roomExitsChangedHandler(events.RoomExitsChanged{UserId: viewer.UserId})
	require.Equal(t, 1, roomInfoUpdates(viewer.UserId))
}

// Ruling 4: leaving the room clears the marks, so coming back shows none.
func TestRoomChange_ClearsDarkMarks(t *testing.T) {
	viewer, _ := watchedViewer(t, 0)
	buttons.RecordDarkMark(viewer.UserId, 9720, "north", buttons.MarkWall)
	(&GMCPRoomModule{}).roomChangeHandler(events.RoomChange{UserId: viewer.UserId, FromRoomId: 9720, ToRoomId: 9721})
	require.Empty(t, buttons.DarkMarks(viewer.UserId, 9720, true))
}

// A despawn forgets the player's ids and the record of what was sent.
func TestRoomExitsWatch_DespawnForgets(t *testing.T) {
	viewer, _ := watchedViewer(t, 60)
	p := roomInfoFor(t, viewer)
	id := p.ExitsV2["northeast"].Id
	sendRoomInfo(viewer)
	(&GMCPRoomModule{}).despawnHandler(events.PlayerDespawn{UserId: viewer.UserId, RoomId: 9720, CharacterName: "Viewer"})
	_, ok := buttons.Resolve(viewer.UserId, 9720, id)
	require.False(t, ok, "a despawned player's per-room ids are gone")
	// Read the record itself: roomExitsDue would be true anyway, since the
	// forgotten ids are issued afresh and the signature differs.
	lastSentRoomExitsMu.Lock()
	_, recorded := lastSentRoomExits[viewer.UserId]
	lastSentRoomExitsMu.Unlock()
	require.False(t, recorded, "nothing is recorded as sent any more")
	require.True(t, roomExitsDue(viewer))
}
