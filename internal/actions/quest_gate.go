package actions

import (
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Quest gates apply to every move that walks through an exit (go, flee) and
// to nothing else: teleports, system moves and the ferry are exempt (owner
// ruling 2026-10-26, #588). A room_gate trigger guards a destination room;
// its conditions are read with the player still in the room they are leaving,
// so in_room names the side the gate stands on. The destination matches by
// its template id, as room_enter does, so a gate holds inside instances too.

// QuestGateRefuses runs the room_gate triggers for a step from room into
// destRoom and reports whether one refused it. The triggers' actions run,
// so the refusal is narrated; the caller keeps the player where they are.
// `go` asks this before anything moves.
func QuestGateRefuses(user *users.UserRecord, room *rooms.Room, destRoom *rooms.Room) bool {
	d, bridge := questGateEvent(user, room.RoomId, destRoom.RoomId)
	return questengine.GetEngine().Notify("room_gate", d, bridge, bridge).Handled
}

// QuestGateBlocks reports whether QuestGateRefuses would refuse the same
// step, without running any action: nothing is narrated or granted. Flee
// asks it of each candidate exit so it never picks one the player could not
// walk.
func QuestGateBlocks(user *users.UserRecord, room *rooms.Room, destRoom *rooms.Room) bool {
	return questGateBlocksRoomId(user, room.RoomId, destRoom.RoomId)
}

// questGateBlocksRoomId is QuestGateBlocks by room ids, for an exit whose
// destination room is not loaded.
func questGateBlocksRoomId(user *users.UserRecord, roomId, destRoomId int) bool {
	d, bridge := questGateEvent(user, roomId, destRoomId)
	return questengine.GetEngine().WouldFire("room_gate", d, bridge)
}

func questGateEvent(user *users.UserRecord, roomId, destRoomId int) (questengine.EventDetails, *questengine.GameBridge) {
	matchRoom, _ := rooms.OriginalRoomId(destRoomId)
	return questengine.EventDetails{UserId: user.UserId, RoomId: matchRoom},
		questengine.NewGameBridge(user, roomId)
}
