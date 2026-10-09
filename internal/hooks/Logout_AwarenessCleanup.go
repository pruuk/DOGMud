package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Logout_AwarenessCleanup forces Awareness through Revealing →
// Visible synchronously when a player logs out or disconnects.
// Ensures hidden players don't leave the world Hidden; on
// reconnect they're Visible. Room broadcast fires so observers
// in the room see the leave.
//
// Registered at default priority so it fires before HandleLeave
// (events.Last), while the UserRecord is still in the manager.
//
// Mob despawn is handled by instance destruction — the
// Awareness machine on the Character goes with the instance,
// so no separate cascade is needed.
func init() {
	events.RegisterListener(events.PlayerDespawn{}, onPlayerDespawnForAwareness)
}

func onPlayerDespawnForAwareness(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.PlayerDespawn)
	if !ok {
		return events.Continue
	}
	u := users.GetByUserId(evt.UserId)
	if u == nil {
		return events.Continue
	}
	// #456: HandleLeave's quit line runs after this, when the quitter is
	// visible to everyone. Record now who could NOT make them out, so the
	// line skips those readers. nil (no one) deletes the key.
	var unseenBy []int
	if room := rooms.LoadRoom(u.Character.RoomId); room != nil {
		unseenBy = playersNotPerceiving(room, u.Character)
	}
	if len(unseenBy) == 0 {
		u.SetTempData(despawnUnseenByKey, nil)
	} else {
		u.SetTempData(despawnUnseenByKey, unseenBy)
	}
	u.Character.Awareness.ForceVisible(state.TransitionReason{
		Trigger: awareness.TriggerLogout,
		Actor:   state.ActorRef{UserId: evt.UserId},
	})
	return events.Continue
}

// despawnUnseenByKey is the user temp-data key onPlayerDespawnForAwareness
// sets for HandleLeave: the ids of the players in the quitter's room who did
// not perceive them before logout forced them visible.
const despawnUnseenByKey = `despawnUnseenBy`

// playersNotPerceiving lists the players in room who cannot make out c
// (characters.Character.Perceives: c is hidden and they have no see-hidden).
func playersNotPerceiving(room *rooms.Room, c *characters.Character) []int {
	var ids []int
	for _, uid := range room.GetPlayers() {
		if v := users.GetByUserId(uid); v != nil && !v.Character.Perceives(c) {
			ids = append(ids, uid)
		}
	}
	return ids
}
