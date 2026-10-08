package aicompanion

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// #276: a companion's loot line named the corpse (and her) to every reader
// through a plain SendTextVisual; a mob-corpse tag is not an identity tag, so
// Anonymize left the dead one's name. It now goes out like the player's
// `loot` room line (#428): ObservedName with both names hidden per reader.

const lootSightInfraredConditionId = 96601

// companionLootLines has Mara (charmed to Corvin) loot a gold-only corpse of
// Deadric and returns what the passer-by Bram read, tags stripped.
func companionLootLines(t *testing.T, dark bool) []string {
	t.Helper()
	_, other, room, her := harmWorld(t, configs.PVPDisabled)
	enabled := module.cfg.Enabled
	module.cfg.Enabled = true
	t.Cleanup(func() { module.cfg.Enabled = enabled })
	her.Character.Charm(1, -1, ``)

	if dark {
		t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
			lootSightInfraredConditionId: {
				ConditionId: lootSightInfraredConditionId,
				Name:        "Test Heat Eyes",
				Flags:       []conditions.Flag{conditions.InfraredVision},
				Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}},
			},
		}))
		room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
		if !other.Character.Conditions.AddCondition(lootSightInfraredConditionId, true) {
			t.Fatal("precondition: Bram should now carry infrared")
		}
	}

	corpse := rooms.Corpse{MobId: 12, RoundCreated: 5, Loot: rooms.Container{Gold: 7}}
	corpse.Character.Name = "Deadric"
	room.Corpses = []rooms.Corpse{corpse}
	events.DrainQueuedMessagesForTest(other.UserId)

	if _, err := mobCompanionLoot(corpseRef(&room.Corpses[0]), her, room); err != nil {
		t.Fatal(err)
	}
	if room.Corpses[0].Loot.Gold != 0 {
		t.Fatal("precondition: she should have taken the gold")
	}
	var out []string
	for _, l := range events.DrainQueuedMessagesForTest(other.UserId) {
		out = append(out, strings.TrimSpace(ansiTag.ReplaceAllString(l, ``)))
	}
	return out
}

func TestCompanionLootLineHidesNamesFromAShapesReader(t *testing.T) {
	got := companionLootLines(t, true)
	if len(got) != 1 || got[0] != "A figure loots the corpse of a figure." {
		t.Fatalf("a shapes-only watcher read %q, want neither name", got)
	}
}

func TestCompanionLootLineNamesBothToAClearReader(t *testing.T) {
	got := companionLootLines(t, false)
	if len(got) != 1 || got[0] != "Mara loots the corpse of Deadric." {
		t.Fatalf("a clear-sighted watcher read %q", got)
	}
}
