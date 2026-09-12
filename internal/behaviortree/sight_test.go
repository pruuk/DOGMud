package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

const (
	sightNightVisionBuffId  = 29
	sightIlluminationBuffId = 1
)

// sightScene stands one mob (instance 8101) in room 8100 of the given biome.
// SpeciesId is 1 deliberately: species.GetSpecies() returns nil otherwise and
// several mob paths dereference it.
func sightScene(t *testing.T, biome string) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", DarkArea: true},
		"city": {BiomeId: "city", LitArea: true},
	}))
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		sightNightVisionBuffId:  {BuffId: sightNightVisionBuffId, Name: "Night Vision", Flags: []buffs.Flag{buffs.NightVision}},
		sightIlluminationBuffId: {BuffId: sightIlluminationBuffId, Name: "Illumination", Flags: []buffs.Flag{buffs.EmitsLight}},
	}))

	room := &rooms.Room{RoomId: 8100, Biome: biome}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{8100: room}, map[string]*rooms.ZoneConfig{}))

	m := &mobs.Mob{
		MobId:      8000,
		InstanceId: 8101,
		HomeRoomId: 8100,
		Character: characters.Character{
			Name:      "Watcher",
			RoomId:    8100,
			Health:    100,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
			SpeciesId: 1,
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

	require.NoError(t, m.Character.AddBuff(sightNightVisionBuffId, true))
	require.True(t, mobCanSee(m, room), "night vision restores sight in the dark")
}

func TestMobCanSeeLitRoom(t *testing.T) {
	m, room := sightScene(t, "city")
	require.True(t, mobCanSee(m, room))
}

// These run on every tree tick; a missing instance must not blind the world.
func TestMobCanSeeDefaultsOpen(t *testing.T) {
	require.True(t, mobCanSee(nil, nil))
}
