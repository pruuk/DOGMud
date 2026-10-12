package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// announceCorpse tells the room a body was left behind, only to readers who
// can see it (M6 slice 3, ledger row 15): a reader who sees the room clearly
// reads the named line, a reader who makes out only shapes reads the
// anonymous one, and a reader who sees nothing reads nothing (the collapse
// sound already told them). Called only where a corpse is created.
//
// identity is the tagged name (`<ansi fg="mobname">Name</ansi>` for a mob, the
// `username` tag for a player; never the corpse's own tag). No existing room
// sender swaps whole text per sight tier (SendTextVisualHidingNames only
// rewrites names, which would read "A figure's body"), so this judges each
// player with messaging.ReaderSight and delivers through SendTextVisualToUser.
func announceCorpse(room *rooms.Room, identity string, excludeUserIds ...int) {
	if room == nil {
		return
	}
	for _, uid := range room.GetPlayers() {
		if corpseLineExcluded(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		switch messaging.ReaderSight(u.Character, room) {
		case messaging.SightFull:
			room.SendTextVisualToUser(u, messaging.CategoryDeath,
				identity+"'s body lies where it fell.")
		case messaging.SightShapes:
			room.SendTextVisualToUser(u, messaging.CategoryDeath,
				"A body lies where it fell.")
		}
	}
}

func corpseLineExcluded(uid int, excluded []int) bool {
	for _, e := range excluded {
		if e == uid {
			return true
		}
	}
	return false
}
