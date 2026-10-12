package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// announceCorpse tells the room remains were left behind, only to readers who
// can see them (M6 slice 3, ledger row 15): a reader who sees the room clearly
// reads the named line, a reader who makes out only shapes reads the
// anonymous one, and a reader who sees nothing reads nothing (the collapse
// sound already told them). Called only where a corpse is created. The lines
// say "remains", not "body", so they fit wreckage, an elemental, an orb or a
// slime as well as a beast.
//
// identity is the tagged name (`<ansi fg="mobname">Name</ansi>` for a mob, the
// `username` tag for a player; never the corpse's own tag). An empty identity
// sends the anonymous line to full sight too: a mob with an authored
// corpse_name (a wagon's wreckage) is not "<Name>'s remains". No existing room
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
			if identity == "" {
				room.SendTextVisualToUser(u, messaging.CategoryDeath, corpseLineAnonymous)
				continue
			}
			room.SendTextVisualToUser(u, messaging.CategoryDeath,
				identity+"'s remains lie where they fell.")
		case messaging.SightShapes:
			room.SendTextVisualToUser(u, messaging.CategoryDeath, corpseLineAnonymous)
		}
	}
}

// corpseLineAnonymous is the corpse line that names no one.
const corpseLineAnonymous = "Some remains lie where they fell."

func corpseLineExcluded(uid int, excluded []int) bool {
	for _, e := range excluded {
		if e == uid {
			return true
		}
	}
	return false
}
