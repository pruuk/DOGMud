package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Forage is a thin wrapper over actions.Forage. The action handles
// biome check, cooldown, score calculation, rendering, and item
// store. This wrapper handles the quest-engine command notification.
func Forage(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if refuseWhileBusy(user, `forage`) {
		return true, nil
	}
	actor := &actions.UserActor{User: user, Room: room}
	result := actions.Forage(actor, actions.ForageOptions{})

	// Quest engine: the forage ran. A forage refused for the ground or the
	// cooldown sends nothing, so a quest step cannot finish on a refusal
	// (owner ruling 2026-10-17). A search that turns up nothing still counts:
	// the player did forage.
	if !result.RollHappened {
		return true, nil
	}
	questengine.NotifyCommand(user, room.RoomId, "forage")

	return true, nil
}
