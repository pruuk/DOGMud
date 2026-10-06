package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// MobDeathItemProcs records the last-kill round (the Blackrazor hunger
// anchor, Task 11) and fires on_kill into every worn treed item, for every
// player with damage attribution on the kill. The one event carries both
// kill lines and on_kill procs (ruling S4).
func MobDeathItemProcs(e events.Event) events.ListenerReturn {
	evt, typeOk := e.(events.MobDeath)
	if !typeOk {
		return events.Continue
	}
	for uid := range evt.PlayerDamage {
		user := users.GetByUserId(uid)
		if user == nil {
			continue
		}
		user.Character.SetMiscData("pinnacle_last_kill_round", util.GetRoundCount())

		// Every worn treed item hears of the kill (item behaviour slice 2,
		// ruling S3: the shield's kill lines too, not the weapon's alone).
		// Its speak node is paced by the item's cooldown, so a multi-kill
		// round does not spam, and gated by PinnacleItemsEnabled. An
		// on_kill proc branch rides the same event, gated by
		// ItemProcsEnabled (ruling S4: any worn item, not the weapon alone).
		fireWornItemEvent(behaviortree.EventContext{EventType: "on_kill"}, user.Character, uid, 0)
	}
	return events.Continue
}
