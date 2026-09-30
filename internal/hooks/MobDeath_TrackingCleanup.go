package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const activeTrackingCondition = 86

// MobDeathTrackingCleanup clears tracking/shadow state on any character
// (player or mob) that was tracking or shadowing the now-dead mob. Pairs
// with PlayerDespawn_TrackingCleanup for the symmetric logoff path.
//
// State cleared per pointing character:
//   - tracking-mob misc data (string match on CharacterName)
//   - tracking-display-count misc data (cleared alongside tracking-mob)
//   - condition 86 (Active Tracking) — only if tracking-mob pointed at this mob
//   - the shadow (actions.ClearShadow: target and Shadowing condition, no
//     cooldown, no message), only if actions.ShadowTargetOf names this mob
func MobDeathTrackingCleanup(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.MobDeath)
	if !ok {
		return events.Continue
	}

	// CharacterName is populated at death time on the event payload;
	// no need to fetch the instance or template.
	dyingName := evt.CharacterName
	dyingInstanceId := evt.InstanceId

	clearPointers := func(c *characters.Character) {
		// Tracking by name.
		if dyingName != "" {
			if v := c.GetMiscData("tracking-mob"); v != nil {
				if s, ok := v.(string); ok && s == dyingName {
					c.SetMiscData("tracking-mob", nil)
					c.SetMiscData("tracking-display-count", nil)
					c.RemoveCondition(activeTrackingCondition)
				}
			}
		}
		// Shadow by InstanceId.
		if _, mobInstanceId := actions.ShadowTargetOf(c); mobInstanceId != 0 && mobInstanceId == dyingInstanceId {
			actions.ClearShadow(c)
		}
	}

	// Walk all online users.
	for _, u := range users.GetAllActiveUsers() {
		if u == nil {
			continue
		}
		clearPointers(u.Character)
	}

	// Walk all active mob instances.
	for _, instId := range mobs.GetAllMobInstanceIds() {
		if instId == dyingInstanceId {
			continue
		}
		m := mobs.GetInstance(instId)
		if m == nil {
			continue
		}
		clearPointers(&m.Character)
	}

	return events.Continue
}
