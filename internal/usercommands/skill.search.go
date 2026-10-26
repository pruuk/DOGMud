package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Search is a thin wrapper over actions.Search. The action handles
// all tier rolls, template rendering, cooldown gating, and skill
// progression.
//
// `search` searches the room; `search <feature>` searches one thing in it
// (a noun, a discovered hidden noun or a visible container). See
// actions/search_feature.go.
func Search(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if refuseWhileBusy(user, `search`) {
		return true, nil
	}
	actor := &actions.UserActor{User: user, Room: room}
	// A search refused for cooldown never ran, so it sends no quest event: a
	// quest step advances only when the action succeeds (owner, 2026-10-17).
	result := actions.Search(actor, actions.SearchOptions{Feature: rest})
	if result.OnCooldown {
		return true, nil
	}

	// #375 phase 3a: `search <feature>` raises the search-family
	// room_interact after the search has run, so a `verb: search` trigger on
	// that feature fires with no roll. Only a resolved feature counts (a room
	// noun, a discovered hidden noun or a visible container, as the search
	// itself resolved it). Plain search's quest rolls are Task 7's.
	if result.Feature != `` {
		notifyRoomInteract(user, room, quests.VerbSearch, rest, result.Feature)
	}

	// Quest engine: command notification — lets a quest gate a step on the
	// `search` command (the newbie Lore spoke's shrine beat: searching out the
	// hidden Orbital Stone in the Reliquary). Mirrors forage/drink/throw/cast.
	// Sent for a feature search too, as before for every search: a player led
	// to the Reliquary may well type `search floor` rather than `search`.
	bridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "search",
	}, bridge, bridge)

	return true, nil
}
