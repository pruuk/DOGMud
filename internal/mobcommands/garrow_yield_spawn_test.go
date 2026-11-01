package mobcommands

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

const (
	garrowMobId  = 9158
	garrowRoomId = 5335
	garrowViewer = 7451
)

// garrowShippedSpawn reads the Mustering Green's real spawn entry for Garrow.
func garrowShippedSpawn(t *testing.T, world string) rooms.SpawnInfo {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(world, "rooms", "pothole_coulee", "5335.yaml"))
	require.NoError(t, err)
	var room struct {
		SpawnInfo []rooms.SpawnInfo `yaml:"spawninfo"`
	}
	require.NoError(t, yaml.Unmarshal(data, &room))
	for _, s := range room.SpawnInfo {
		if s.MobId == garrowMobId {
			return s
		}
	}
	t.Fatal("the Mustering Green does not spawn Garrow")
	return rooms.SpawnInfo{}
}

// garrowShippedExit reads the command his yield branch ends on, the one that
// takes him off the green.
func garrowShippedExit(t *testing.T, world string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(world, "behaviors", "pothole_coulee", "9158-sullen_garrow.yaml"))
	require.NoError(t, err)
	var doc struct {
		Tree struct {
			Children []struct {
				Event    string `yaml:"event"`
				Children []struct {
					Do  string `yaml:"do"`
					Cmd string `yaml:"cmd"`
				} `yaml:"children"`
			} `yaml:"children"`
		} `yaml:"tree"`
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))
	for _, br := range doc.Tree.Children {
		if br.Event != "mob_combat_round" {
			continue
		}
		last := br.Children[len(br.Children)-1]
		require.Equal(t, "command", last.Do)
		return last.Cmd
	}
	t.Fatal("Garrow's tree has no mob_combat_round branch")
	return ""
}

// garrowOnTheGreen seeds a lit green with a watching player and the given
// spawn entry, spawns Garrow by the real Prepare, and returns his instance.
func garrowOnTheGreen(t *testing.T, spawn rooms.SpawnInfo) (*rooms.Room, *mobs.Mob) {
	t.Helper()
	t.Cleanup(keywords.SeedKeywordsForTest())
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		garrowMobId: {MobId: garrowMobId, Zone: "Pothole Coulee", Character: characters.Character{Name: "Sullen Garrow"}},
	}, map[int]*mobs.Mob{}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{"city": {BiomeId: "city"}, "default": {BiomeId: "default"}}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		garrowViewer: users.NewTestUser(garrowViewer, "ferdson", "Ferdson", 97451),
	}))
	room := &rooms.Room{RoomId: garrowRoomId, Zone: "Pothole Coulee", Biome: "city", SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60)}
	room.SpawnInfo = []rooms.SpawnInfo{spawn}
	room.AddPlayer(garrowViewer)
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{garrowRoomId: room}, map[string]*rooms.ZoneConfig{}))
	t.Cleanup(util.ResetRoundCountForTest)
	events.DrainQueuedMessagesForTest(garrowViewer)

	room.Prepare(false)
	ids := room.GetMobs()
	require.Len(t, ids, 1, "the green spawns Garrow")
	require.Empty(t, events.DrainQueuedMessagesForTest(garrowViewer), "a first spawn is quiet")
	mob := mobs.GetInstance(ids[0])
	require.NotNil(t, mob)
	mob.HomeRoomId = garrowRoomId
	mob.Character.RoomId = garrowRoomId
	return room, mob
}

// runMobLine runs one full command line the way a queued mob input does.
func runMobLine(t *testing.T, line string, instanceId int) {
	t.Helper()
	cmd, rest, _ := strings.Cut(line, " ")
	handled, err := TryCommand(cmd, rest, instanceId)
	require.NoError(t, err)
	require.True(t, handled, "%q must be a mob command", line)
}

// Quest 48: after his yield Garrow stays gone for his spawn entry's rate and
// comes back with its line. The tree used `despawn`, which resets the spawn
// round, so he was back a round after "quits the green", with no line.
func TestGarrowsYieldKeepsHimGoneForHisRateThenNarratesHisReturn(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	world := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud")
	spawn := garrowShippedSpawn(t, world)
	require.Equal(t, "5 rounds", spawn.RespawnRate)
	require.True(t, spawn.NarrateRespawn)
	require.NotEmpty(t, spawn.Message)

	room, mob := garrowOnTheGreen(t, spawn)
	start := util.GetRoundCount()
	runMobLine(t, garrowShippedExit(t, world), mob.InstanceId)
	require.Empty(t, room.GetMobs(), "his yield takes him off the green")
	require.Nil(t, mobs.GetInstance(mob.InstanceId))

	// HandleRespawns prepares an occupied room every round.
	for offset := uint64(0); offset < 5; offset++ {
		util.SetRoundCountForTest(start + offset)
		room.Prepare(false)
		require.Empty(t, room.GetMobs(), "he is still gone %d rounds after his yield", offset)
		require.Empty(t, events.DrainQueuedMessagesForTest(garrowViewer))
	}

	util.SetRoundCountForTest(start + 5)
	room.Prepare(false)
	ids := room.GetMobs()
	require.Len(t, ids, 1, "he is back once the rate has passed")
	t.Cleanup(func() { mobs.SetInstanceForTest(ids[0], nil) })
	lines := events.DrainQueuedMessagesForTest(garrowViewer)
	require.Len(t, lines, 1, "the green hears him come back")
	require.Contains(t, lines[0], spawn.Message)
}

// `despawn` keeps its old meaning for its other callers: a presence unload or
// an expired charm is no death, so the mob's slot is ready at once and returns
// quietly the next time the room is prepared.
func TestDespawnStillLeavesTheSlotReadyAtOnce(t *testing.T) {
	spawn := rooms.SpawnInfo{MobId: garrowMobId, Message: "He is back.", NarrateRespawn: true, RespawnRate: "5 rounds"}
	room, mob := garrowOnTheGreen(t, spawn)
	runMobLine(t, "despawn presence_despawning", mob.InstanceId)
	require.Empty(t, room.GetMobs())

	room.Prepare(false)
	ids := room.GetMobs()
	require.Len(t, ids, 1, "a despawned mob's slot has no cooldown")
	t.Cleanup(func() { mobs.SetInstanceForTest(ids[0], nil) })
	require.Empty(t, events.DrainQueuedMessagesForTest(garrowViewer), "and it returns quietly")
}
