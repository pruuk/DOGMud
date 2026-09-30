package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/housing"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Sleep is the player-facing sleep verb. Delegates to actions.Sleep
// which applies the Sleeping condition (chunk 3.3, condition id 15).
func Sleep(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if refuseWhileBusy(user, `sleep`) {
		return true, nil
	}
	// A housing bed doubles the rest (anyone let into the lodging may use it).
	actions.Sleep(&actions.UserActor{User: user, Room: room}, actions.SleepOptions{InBed: housing.RoomHasBed(room.RoomId)})

	// Quest engine: command notification — sleeping advances "rest in the
	// field" quest steps (e.g. the Spoke D wilderness cert). Mirrors the
	// drink/throw/forage command Notify pattern.
	bridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "sleep",
	}, bridge, bridge)

	return true, nil
}
