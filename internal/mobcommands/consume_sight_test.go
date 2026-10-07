package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #428 review sweep: a flesh golem's consume line named the corpse inside a
// mob-corpse tag through a plain SendTextVisual, so a shapes-only watcher read
// the dead one's name (Anonymize leaves a non-identity tag alone).

// consumeSightScene makes mob 100 (Skeleton) a flesh golem over a Merchant
// corpse in its room, unlit or lit by a lamp, with Aliceia (user 1) watching
// with heat sight.
func consumeSightScene(t *testing.T, lit bool) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", SkyLight: rooms.SkyLightPtr(0.0), MovementCost: 1},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9001: {ConditionId: 9001, Name: "Test Infrared", RoundInterval: 1, TriggerCount: 1, Flags: []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	mob, room := getTestMobAndRoom(t)
	origSpecies := mob.Character.SpeciesId
	mob.Character.SpeciesId = fleshGolemSpeciesId
	origBiome, origSky, origLamp, origCorpses := room.Biome, room.SkyLight, room.Lamp, room.Corpses
	t.Cleanup(func() {
		mob.Character.SpeciesId = origSpecies
		room.Biome, room.SkyLight, room.Lamp, room.Corpses = origBiome, origSky, origLamp, origCorpses
	})
	room.Biome, room.SkyLight, room.Lamp = "cave", rooms.SkyLightPtr(0), nil
	if lit {
		room.Lamp = rooms.LampPtr(60)
	}
	corpse := rooms.Corpse{MobId: 2}
	corpse.Character.Name = "Merchant"
	room.Corpses = []rooms.Corpse{corpse}
	require.True(t, users.GetByUserId(1).Character.Conditions.AddCondition(9001, true))
	room.AddPlayer(1)
	events.DrainQueuedMessagesForTest(1)
	return mob, room
}

func TestGolemConsume_ShapesWatcherDoesNotReadTheCorpseName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := consumeSightScene(t, false)
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(users.GetByUserId(1).Character, room))

	_, err := Consume("", mob, room)
	require.NoError(t, err)

	require.Equal(t, []string{"A figure rips a piece from the corpse of a figure and grafts it onto itself! Its\nform grows more massive."},
		mobSpeechHeard(1))
}

func TestGolemConsume_ClearWatcherReadsTheNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := consumeSightScene(t, true)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(users.GetByUserId(1).Character, room))

	_, err := Consume("", mob, room)
	require.NoError(t, err)

	require.Equal(t, []string{"Skeleton rips a piece from the corpse of Merchant and grafts it onto itself! Its\nform grows more massive."},
		mobSpeechHeard(1))
}
