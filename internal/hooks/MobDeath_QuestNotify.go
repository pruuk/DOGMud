package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// MobDeathQuestNotify fires quest engine mob_death notifications for every
// player who gets the kill (killCreditUserIds): each player who damaged the
// dying mob, plus each party member of theirs in the room where it died,
// each once. A follower whose leader's party autoattack landed every blow
// gets the kill too (owner ruling 2026-10-17). This is separate from
// PackFlee so quest triggers work regardless of pack/species behavior.
func MobDeathQuestNotify(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.MobDeath)
	if !ok {
		return events.Continue
	}

	for _, userId := range killCreditUserIds(evt.PlayerDamage, evt.RoomId) {
		if u := users.GetByUserId(userId); u != nil {
			bridge := questengine.NewGameBridge(u, evt.RoomId)
			questengine.GetEngine().Notify("mob_death", questengine.EventDetails{
				UserId: userId,
				RoomId: evt.RoomId,
				MobId:  evt.MobId,
			}, bridge, bridge)
		}
	}

	return events.Continue
}
