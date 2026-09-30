package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
)

// A quarry's death or logoff drops every shadow on it through
// actions.ClearShadow: target and condition 87 gone, no cooldown, no line
// (owner ruling D2). A shadow on someone else is untouched.
func TestTrackingCleanup_DropsShadowsOnTheDepartedQuarry(t *testing.T) {
	for _, shadower := range shadowKinds {
		t.Run(string(shadower)+" shadower, mob quarry dies", func(t *testing.T) {
			s := newShadowScene(t)
			c := s.sly(shadower)
			s.shadowTarget(c, shadowMob, false, true, true)
			other := s.quarryUser.Character // shadows nobody in this scene
			s.shadowTarget(other, shadowMob, true, true, false)

			MobDeathTrackingCleanup(events.MobDeath{InstanceId: shadowQuarryMob, CharacterName: "Stag"})

			if !shadowEnded(c) {
				t.Error("the shadow on the dead mob survived")
			}
			if shadowCooldown(c) != 0 {
				t.Error("the death cleanup started a cooldown")
			}
			if _, mobInstanceId := actions.ShadowTargetOf(other); mobInstanceId != shadowNobody {
				t.Error("a shadow on another mob was cleared")
			}
			if got := len(events.DrainQueuedMessagesForTest(shadowSlyUser)); got != 0 {
				t.Errorf("the death cleanup sent the shadower %d lines", got)
			}
		})
		t.Run(string(shadower)+" shadower, player quarry logs off", func(t *testing.T) {
			s := newShadowScene(t)
			c := s.sly(shadower)
			s.shadowTarget(c, shadowPlayer, false, true, true)

			PlayerDespawnTrackingCleanup(events.PlayerDespawn{UserId: shadowQuarryUser, CharacterName: "Quarry"})

			if !shadowEnded(c) {
				t.Error("the shadow on the departed player survived")
			}
			if shadowCooldown(c) != 0 {
				t.Error("the logoff cleanup started a cooldown")
			}
			if got := len(events.DrainQueuedMessagesForTest(shadowSlyUser)); got != 0 {
				t.Errorf("the logoff cleanup sent the shadower %d lines", got)
			}
		})
	}
}
