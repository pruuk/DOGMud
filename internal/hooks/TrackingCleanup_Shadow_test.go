package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
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

			// #275: the record is dropped, not expired, so the prune pass
			// tells no "Your focus on the quarry breaks." end line.
			for _, pruned := range c.Conditions.Prune() {
				if pruned.ConditionId == actions.ShadowingConditionId {
					t.Error("the logoff expired the shadow, so its end line would print")
				}
			}
			if !shadowEnded(c) {
				t.Error("the shadow on the departed player survived")
			}
			if shadowCooldown(c) != 0 {
				t.Error("the logoff cleanup started a cooldown")
			}
			got := events.DrainQueuedMessagesForTest(shadowSlyUser)
			if shadower == shadowMob {
				if len(got) != 0 {
					t.Errorf("a mob shadower's logoff cleanup sent the sly user %d lines", len(got))
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], logoffQuarryLine("Quarry")) {
				t.Errorf("the shadower read %q, want one line %q", got, logoffQuarryLine("Quarry"))
			}
		})
	}
}

// #275, its tracking sibling: a tracker whose quarry logs off reads the same
// one line, and condition 86 is dropped without its end line.
func TestPlayerDespawnTrackingCleanup_TrackerReadsTheQuarryRetired(t *testing.T) {
	s := newShadowScene(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		activeTrackingCondition: {ConditionId: activeTrackingCondition, Name: "Active Tracking", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25},
	}))
	c := s.sly(shadowPlayer)
	c.SetMiscData("tracking-user", "Quarry")
	if err := c.AddCondition(activeTrackingCondition, false); err != nil {
		t.Fatalf("precondition: the tracker should hold Active Tracking: %v", err)
	}
	events.DrainQueuedMessagesForTest(shadowSlyUser)

	PlayerDespawnTrackingCleanup(events.PlayerDespawn{UserId: shadowQuarryUser, CharacterName: "Quarry"})

	for _, pruned := range c.Conditions.Prune() {
		if pruned.ConditionId == activeTrackingCondition {
			t.Error("the logoff expired Active Tracking, so its end line would print")
		}
	}
	if c.HasCondition(activeTrackingCondition) || c.GetMiscData("tracking-user") != nil {
		t.Error("the tracking on the departed player survived")
	}
	got := events.DrainQueuedMessagesForTest(shadowSlyUser)
	if len(got) != 1 || !strings.Contains(got[0], logoffQuarryLine("Quarry")) {
		t.Errorf("the tracker read %q, want one line %q", got, logoffQuarryLine("Quarry"))
	}
}
