package behaviortree

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// The Chitin Throne as shipped (lighting plan 5d, Rule 6): room 508 spawns the
// Chrysalis Phantom (272) through the real spawn path (Room.Prepare), which
// lists the mob without trimming, so its Umbral Lantern is at full strength
// and the lair reads -50. Its heat sense (condition 133, reach 50) reads
// shapes at exactly -50, so under ruling D8 it can act; a normal player and a
// normal-eyed mob there read nothing.
func TestThePhantomsLairIsBlack(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	// fileloader and ReloadConfig resolve "_datafiles/..." against the CWD,
	// which a shared test binary does not set to this package's directory
	// (dogmud-writing-tests). Anchor on this file and chdir to the repo root.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	require.NoError(t, err)
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoRoot))
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// The real shipped config (config.yaml): a bare test binary would read
	// Network.LogoutRounds as 0 and conditions.LoadDataFiles would panic on
	// condition 0. SetConfigForTest restores the pre-test config afterwards.
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())

	// Each loader replaces its registry with no restore of its own; seeding a
	// throwaway first captures the pre-test registry for a real restore.
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(species.SeedSpeciesForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(rooms.SeedBiomesForTest(nil))
	conditions.LoadDataFiles()
	species.LoadDataFiles()
	items.LoadDataFiles()
	rooms.LoadBiomeDataFiles()

	dataFiles := string(configs.GetFilePathsConfig().DataFiles)
	thornwall, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataFiles + `/mobs/thornwall_city`)
	require.NoError(t, err)
	phantomSpec, ok := thornwall[272]
	require.True(t, ok, "mob 272 (Chrysalis Phantom) is not shipped")
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{272: phantomSpec}, map[int]*mobs.Mob{}))

	raw, err := os.ReadFile(dataFiles + `/rooms/thornwall_city/508.yaml`)
	require.NoError(t, err)
	lair := &rooms.Room{}
	require.NoError(t, yaml.Unmarshal(raw, lair))
	require.Equal(t, 508, lair.RoomId, "read the wrong room file, so this test proves nothing")
	require.Equal(t, "cave", lair.Biome)
	require.Nil(t, lair.Lamp, "the lair must have no lamp of its own")
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{508: lair},
		map[string]*rooms.ZoneConfig{lair.Zone: {Name: lair.Zone, RoomId: 508, RoomIds: map[int]struct{}{508: {}}}}))

	require.Equal(t, 0, lair.LightLevel(), "before the spawn the lair is an unlit cave")
	lair.Prepare(false)
	mobIds := lair.GetMobs()
	require.Len(t, mobIds, 1, "room 508 must spawn exactly its Phantom")
	phantom := mobs.GetInstance(mobIds[0])
	require.NotNil(t, phantom)
	t.Cleanup(func() { mobs.SetInstanceForTest(mobIds[0], nil) })
	require.Equal(t, 272, int(phantom.MobId))

	require.Equal(t, 20098, phantom.Character.Equipment.Light.ItemId, "the Phantom carries the Umbral Lantern in its light slot")
	require.Equal(t, 25, phantom.Character.Equipment.Light.DropChance, "the lantern drops one kill in four")
	require.Equal(t, -50, lair.LightLevel(), "the lantern is at full strength after a spawn: 0 before, -50 after")
	require.Equal(t, 50, phantom.Character.InfraReach(), "the Phantom's heat sense reaches 50")
	require.Equal(t, messaging.BandShapes, messaging.LightBand(&phantom.Character, lair), "the Phantom reads shapes in its own dark")
	require.True(t, mobCanSee(phantom, lair), "ruling D8: the Phantom acts on what it makes out, so its ambush fires")

	player := users.NewTestUser(8140, "delver", "Delver", 98140)
	require.Equal(t, messaging.BandDark, messaging.LightBand(player.Character, lair), "a normal player sees nothing in the lair")

	normal := &mobs.Mob{MobId: 8141, InstanceId: 8142,
		Character: characters.Character{Name: "Rat", RoomId: 508, Conditions: conditions.New(), SpeciesId: 1}}
	require.False(t, mobCanSee(normal, lair), "a normal-eyed mob at -50 is below the window floor: D8 does not reach it")
}
