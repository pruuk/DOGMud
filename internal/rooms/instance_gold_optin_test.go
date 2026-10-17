package rooms

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Owner ruling 2026-10-17 (second half): instance gold is opt-in per zone.
// A paid instance whose zone-config does not opt in (Crash Site Interior
// today) keeps template gold exactly as before. These tests drive the real
// CreateZoneInstance path: zone config, clone, scale, stamp, then spawn.

const (
	optInEntryRoomId = 9011
	optInMobId       = 7871
	optInTemplate    = 7
)

// paidInstanceSpawnGold builds a one-room instanced zone whose room spawns a
// boss-tier (pool 4) mob with template gold 7, opens it for goldPaid through
// CreateZoneInstance, spawns the room and returns the mob's gold. zCfg is
// filled in with the fixture's name, entry room and room ids.
func paidInstanceSpawnGold(t *testing.T, zCfg *ZoneConfig, goldPaid int) int {
	t.Helper()
	pinInstanceGoldConfig(t, 50000)

	const zoneFolder = "test_optin_zone"
	zCfg.Name = "Test Opt-in Zone"
	zCfg.Instanced = true
	zCfg.EntryRoom = optInEntryRoomId
	zCfg.DeathPolicy = "rejoin"
	zCfg.PortalDuration = "30 real minutes"
	zCfg.RoomIds = map[int]struct{}{optInEntryRoomId: {}}

	tempDir := t.TempDir()
	roomsDir := filepath.Join(tempDir, "rooms", zoneFolder)
	require.NoError(t, os.MkdirAll(roomsDir, 0755))
	roomYAML, err := yaml.Marshal(&Room{
		RoomId:      optInEntryRoomId,
		Zone:        zCfg.Name,
		Title:       "Test Pit",
		Description: "A bare pit used only in tests.",
		Exits:       map[string]exit.RoomExit{},
		SpawnInfo:   []SpawnInfo{{MobId: optInMobId, StatPool: 4}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(roomsDir, "9011.yaml"), roomYAML, 0644))

	prevDataFiles := configs.GetFilePathsConfig().DataFiles.String()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tempDir}))
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prevDataFiles})
	})

	t.Cleanup(SeedRoomsForTest(map[int]*Room{}, map[string]*ZoneConfig{zCfg.Name: zCfg}))
	roomManager.roomIdToFileCache[optInEntryRoomId] = filepath.Join(zoneFolder, "9011.yaml")
	t.Cleanup(func() {
		for i := range ephemeralRoomChunks {
			ephemeralRoomChunks[i] = nil
		}
		originalRoomIdLookups = map[int]int{}
		instanceRegistry = NewInstanceRegistry()
	})

	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		optInMobId: {MobId: optInMobId, Zone: zCfg.Name,
			Character: characters.Character{Name: "Opt-in Probe", Gold: optInTemplate}},
	}, map[int]*mobs.Mob{}))

	inst, err := CreateZoneInstance(zCfg.Name, goldPaid, 1, []int{1}, 1)
	require.NoError(t, err)
	room := LoadRoom(inst.EntryRoomId)
	require.NotNil(t, room)

	restore := util.SetRandForTest(1)
	room.Prepare(false)
	restore()

	ids := room.GetMobs()
	require.Len(t, ids, 1, "the instance room should spawn exactly one mob")
	m := mobs.GetInstance(ids[0])
	require.NotNil(t, m)
	return m.Character.Gold
}

// The same fixture with instance_gold: true scales a boss at 1000 paid to
// 50 gold +-25%.
func TestInstanceGold_PaidZoneThatOptsInScalesGold(t *testing.T) {
	g := paidInstanceSpawnGold(t, &ZoneConfig{InstanceGold: true}, 1000)
	if g < 38 || g > 63 {
		t.Errorf("an opted-in paid instance gave its boss %d gold, want within [38, 63] around 50", g)
	}
}

// The shipped zone-configs: the Arena and the Planar Oasis opt in; Crash Site
// Interior stays out until it is tuned after the quests and dialogue rework
// (owner 2026-10-17). Reads the real YAML, so a dropped or misspelled key (an
// unexported field or a tag typo would unmarshal to false silently) fails.
func TestInstanceGold_ShippedZoneOptIns(t *testing.T) {
	want := map[string]bool{
		"instance_arena":        true,
		"instance_planar_oasis": true,
		"crash_site_interior":   false,
	}
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	for folder, optIn := range want {
		path := filepath.Join(repoRoot, "_datafiles", "world", "dogmud", "rooms", folder, "zone-config.yaml")
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "read %s", path)
		var z ZoneConfig
		require.NoError(t, yaml.Unmarshal(raw, &z), "parse %s", path)
		if z.InstanceGold != optIn {
			t.Errorf("%s: instance_gold = %v, want %v", folder, z.InstanceGold, optIn)
		}
	}
}

func TestInstanceGold_PaidZoneThatDoesNotOptInKeepsTemplateGold(t *testing.T) {
	if g := paidInstanceSpawnGold(t, &ZoneConfig{}, 1000); g != optInTemplate {
		t.Errorf("a paid instance whose zone does not opt in gave its boss %d gold, want its template %d",
			g, optInTemplate)
	}
}
