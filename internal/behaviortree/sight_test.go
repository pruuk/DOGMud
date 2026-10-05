package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

const (
	sightNightVisionConditionId  = 29
	sightIlluminationConditionId = 1
)

// sightScene stands one mob (instance 8101) in room 8100 of the given biome.
// SpeciesId is 1 deliberately: species.GetSpecies() returns nil otherwise and
// several mob paths dereference it.
func sightScene(t *testing.T, biome string) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
		// Lamp pins "city" fully lit regardless of the ambient test round.
		// Since graded lighting plan 3a Task 8, LightLevel() reads the real
		// celestial term (sun + moons at whatever round util.GetRoundCount()
		// holds), which a bare, unpinned test round reads as night with a
		// mixed moon phase (about 28 on the scale, shapes tier) rather than
		// unconditionally lit. A lamp is a room's own light source and always
		// present regardless of time of day, which is exactly the
		// determinism this fixture needs.
		"city": {BiomeId: "city", Lamp: rooms.LampPtr(90)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		sightNightVisionConditionId:  {ConditionId: sightNightVisionConditionId, Name: "Night Vision", Flags: []conditions.Flag{conditions.NightVision}},
		sightIlluminationConditionId: {ConditionId: sightIlluminationConditionId, Name: "Illumination", Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 50}}},
	}))

	room := &rooms.Room{RoomId: 8100, Biome: biome}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{8100: room}, map[string]*rooms.ZoneConfig{}))

	m := &mobs.Mob{
		MobId:      8000,
		InstanceId: 8101,
		HomeRoomId: 8100,
		Character: characters.Character{
			Name:       "Watcher",
			RoomId:     8100,
			Health:     100,
			Conditions: conditions.New(),
			Cooldowns:  map[string]int{},
			SpeciesId:  1,
		},
	}
	m.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(8101, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(8101, nil) })
	return m, room
}

func TestMobCanSeeDarkRoom(t *testing.T) {
	m, room := sightScene(t, "cave")
	require.False(t, mobCanSee(m, room), "an unlit cave blinds a mob with no night vision")

	// GRADED LIGHTING PLAN 2: a shifted window is still blind below its
	// floor at light 0 no matter how strong the shift, so night vision alone
	// does not restore sight in a pitch dark room. Since lighting plan 5d
	// (ruling D8) mobCanSee also accepts shapes, but night vision gives none
	// below the window floor either.
	require.NoError(t, m.Character.AddCondition(sightNightVisionConditionId, true))
	require.False(t, mobCanSee(m, room), "night vision alone still cannot see in true darkness")
}

func TestMobCanSeeLitRoom(t *testing.T) {
	m, room := sightScene(t, "city")
	require.True(t, mobCanSee(m, room))
}

// These run on every tree tick; a missing instance must not blind the world.
func TestMobCanSeeDefaultsOpen(t *testing.T) {
	require.True(t, mobCanSee(nil, nil))
}
