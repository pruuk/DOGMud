package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
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
	// A refused sleep (in combat, or a server fault) sends no quest event:
	// a quest step advances only when the action succeeds (owner, 2026-10-17).
	if !actions.Sleep(&actions.UserActor{User: user, Room: room}, actions.SleepOptions{}).Success {
		return true, nil
	}

	// Quest engine: command notification — sleeping advances "rest in the
	// field" quest steps (e.g. the Spoke D wilderness cert). Mirrors the
	// drink/throw/forage command Notify pattern.
	questengine.NotifyCommand(user, room.RoomId, "sleep")

	return true, nil
}
