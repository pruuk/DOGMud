package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #276: the decay line named the dead one to the whole room on the audio
// channel, whatever each reader could see. It now goes out like every other
// corpse line (#428): ObservedName through SendTextVisualHidingNames with the
// dead one's name hidden. Fixture (sightTestRoom): an unlit cave holding
// Aliceia (7411), Bobrick (7412) and Ordel (7413).
func decayOneCorpse(t *testing.T, r *Room, c Corpse) {
	t.Helper()
	if err := configs.AddOverlayOverrides(map[string]any{"GamePlay.Death.CorpsesEnabled": true}); err != nil {
		t.Fatalf("enable corpses: %v", err)
	}
	t.Cleanup(func() {
		configs.AddOverlayOverrides(map[string]any{"GamePlay.Death.CorpsesEnabled": false})
	})
	c.RoundCreated = 1
	c.Prunable = true
	r.Corpses = []Corpse{c}
	r.UpdateCorpses(999999999)
	if len(r.Corpses) != 0 {
		t.Fatalf("precondition: the corpse should have decayed, %d remain", len(r.Corpses))
	}
}

func TestUpdateCorpses_DecayLineFollowsEachReadersSight(t *testing.T) {
	for _, c := range []struct {
		name   string
		corpse Corpse
		clear  string
	}{
		{"player corpse", Corpse{UserId: 777}, "The corpse of Deadric crumbles to dust."},
		{"mob corpse", Corpse{MobId: 12}, "The corpse of Deadric crumbles to dust."},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := sightTestRoom(t, "cave")
			if !users.GetByUserId(7413).Character.Conditions.AddCondition(sightTestInfraredConditionId, true) {
				t.Fatal("precondition: Ordel should now carry infrared")
			}
			c.corpse.Character.Name = "Deadric"
			decayOneCorpse(t, r, c.corpse)

			shapes := sightTestPlain(events.DrainQueuedMessagesForTest(7413))
			if len(shapes) != 1 || shapes[0] != "The corpse of a figure crumbles to dust." {
				t.Errorf("shapes reader read %q, want exactly the hidden decay line", shapes)
			}
			if blind := events.DrainQueuedMessagesForTest(7412); len(blind) != 0 {
				t.Errorf("a reader who sees nothing read %q", blind)
			}

			lit := sightTestRoom(t, "cave")
			lamp := 90
			lit.Lamp = &lamp
			decayOneCorpse(t, lit, c.corpse)
			clear := sightTestPlain(events.DrainQueuedMessagesForTest(7411))
			if len(clear) != 1 || clear[0] != c.clear {
				t.Errorf("clear reader read %q, want %q", clear, c.clear)
			}
			if strings.Contains(strings.Join(shapes, " "), "Deadric") {
				t.Errorf("the shapes reader learned the name: %q", shapes)
			}
		})
	}
}
