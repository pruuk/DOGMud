package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// QuestRoomEnter raises the quest engine's room_enter event for a player
// arriving in a room, however they got there.
//
// rooms.MoveToRoom queues a RoomChange for every player move, so walking,
// fleeing, teleports (behaviour-tree move_player, the quest bridge, recall,
// portals, the ferry, respawn, admin teleport) and the start vortex all reach
// this one place. Before #619 the event was raised by hand at the walk and
// start sites only, so a room_enter trigger on a teleport destination never
// fired (quest 29's trigger on 468, reached through Esk's arch).
//
// Login does not move the player and is raised by HandleJoin.
//
// For an ephemeral room (tutorial antechamber, dungeons) the trigger matches
// the TEMPLATE id, since the ephemeral id is generated and unauthorable;
// OriginalRoomId is a no-op for a normal room. The bridge keeps the real id so
// npc_say and other actions find mobs and exits in the instance the player is
// actually in.
//
// The listener runs after the code that moved the player returns, so the
// arrival look is already on screen when a trigger's lines are queued.
func QuestRoomEnter(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.RoomChange)
	if !ok || evt.UserId == 0 {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil || user.Character.RoomId != evt.ToRoomId {
		// Moved on again before this was dispatched; that move raises its own.
		return events.Continue
	}

	matchRoom, _ := rooms.OriginalRoomId(evt.ToRoomId)
	bridge := questengine.NewGameBridge(user, evt.ToRoomId)
	questengine.GetEngine().Notify("room_enter", questengine.EventDetails{
		UserId: user.UserId,
		RoomId: matchRoom,
	}, bridge, bridge)

	return events.Continue
}
