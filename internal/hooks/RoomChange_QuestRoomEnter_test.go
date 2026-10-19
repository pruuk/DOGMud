package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #619: a room_enter trigger on a teleport destination never fired, because
// the quest event was raised by hand at the walk and start sites only. It is
// now raised for every RoomChange MoveToRoom queues, so a teleport (here a
// bare MoveToRoom with no exit, as move_player, recall and the ferry do)
// counts.

const roomEnterProbeToken = "999302-start"

func listenForRoomEnter(t *testing.T, u *users.UserRecord, roomId int) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999302,
		Name:    "room enter probe",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:   "room_enter",
			Room:    roomId,
			Actions: []questengine.ActionDef{{Grant: roomEnterProbeToken}},
		}},
	})
	u.Character.ClearQuestToken(roomEnterProbeToken)
	t.Cleanup(func() { u.Character.ClearQuestToken(roomEnterProbeToken) })
}

// dispatchRoomChanges runs QuestRoomEnter over every queued RoomChange, as
// events.ProcessEvents would.
func dispatchRoomChanges(t *testing.T) int {
	t.Helper()
	queued := events.DrainQueuedByTypeForTest(events.RoomChange{}.Type())
	for _, e := range queued {
		QuestRoomEnter(e)
	}
	return len(queued)
}

func TestQuestRoomEnter(t *testing.T) {
	t.Run("a teleport raises room_enter for the destination", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		events.DrainQueuedByTypeForTest(events.RoomChange{}.Type())
		u := users.GetByUserId(1)
		listenForRoomEnter(t, u, 2)

		if err := rooms.MoveToRoom(u.UserId, 2); err != nil {
			t.Fatalf("MoveToRoom: %v", err)
		}
		if n := dispatchRoomChanges(t); n != 1 {
			t.Fatalf("MoveToRoom queued %d RoomChange events, want 1", n)
		}
		if !u.Character.HasQuest(roomEnterProbeToken) {
			t.Fatal("arriving by teleport did not raise room_enter")
		}
	})

	t.Run("a player who has moved on is not entered into the old room", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		events.DrainQueuedByTypeForTest(events.RoomChange{}.Type())
		u := users.GetByUserId(1)
		listenForRoomEnter(t, u, 2)

		QuestRoomEnter(events.RoomChange{UserId: u.UserId, FromRoomId: 1, ToRoomId: 2})
		if u.Character.HasQuest(roomEnterProbeToken) {
			t.Fatal("a stale RoomChange entered the player into a room they are not in")
		}
	})

	t.Run("a mob's move raises nothing", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		u := users.GetByUserId(1)
		listenForRoomEnter(t, u, 1)

		QuestRoomEnter(events.RoomChange{MobInstanceId: 100, FromRoomId: 2, ToRoomId: 1})
		if u.Character.HasQuest(roomEnterProbeToken) {
			t.Fatal("a mob's move raised room_enter for a player")
		}
	})
}
