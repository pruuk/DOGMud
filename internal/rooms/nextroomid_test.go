package rooms

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/require"
)

// useUnwritableNextRoomId installs a config whose DataFiles is a temp tree,
// seeds Server.NextRoomId, and then points the override save path at a missing
// directory so every later SetEngineVal save fails the way prod's read-only
// /mud-config mount does. Returns the temp DataFiles root.
func useUnwritableNextRoomId(t *testing.T, start int) string {
	t.Helper()

	c := configs.GetConfig()
	dataDir := t.TempDir()
	c.FilePaths.DataFiles = configs.ConfigString(dataDir)
	c.Server.Locked = configs.ConfigSliceString{`Server.NextRoomId`}
	configs.SetConfigWithLookupsForTest(t, c)

	require.NoError(t, configs.SetEngineVal(`Server.NextRoomId`, strconv.Itoa(start)))
	require.Equal(t, start, GetNextRoomId())

	t.Setenv(`CONFIG_PATH`, filepath.Join(t.TempDir(), `missing-dir`, `config.yaml`))
	return dataDir
}

func freshRoomManager(t *testing.T) {
	t.Helper()
	orig := roomManager
	roomManager = &RoomManager{
		rooms:             make(map[int]*Room),
		zones:             make(map[string]*ZoneConfig),
		roomsWithUsers:    make(map[int]int),
		roomsWithMobs:     make(map[int]int),
		roomIdToFileCache: make(map[int]string),
	}
	t.Cleanup(func() { roomManager = orig })
}

// (a) With the durable save failing, consecutive SetNextRoomId calls must
// still advance the in-memory counter.
func TestSetNextRoomId_AdvancesInMemoryWhenSaveFails(t *testing.T) {
	useUnwritableNextRoomId(t, 6403)

	SetNextRoomId(6404)
	require.Equal(t, 6404, GetNextRoomId())
	SetNextRoomId(6405)
	require.Equal(t, 6405, GetNextRoomId())
}

// (b) A configured NextRoomId at or below the highest room template on disk
// is raised to max+1 at load, even when the override file cannot be written
// (prod loses NextRoomId across restarts because /mud-config is read-only).
func TestLoadAllRoomZones_RaisesNextRoomIdPastDisk(t *testing.T) {
	dataDir := useUnwritableNextRoomId(t, 100)
	freshRoomManager(t)

	zoneDir := filepath.Join(dataDir, `rooms`, `zone_a`)
	require.NoError(t, os.MkdirAll(zoneDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(zoneDir, `zone-config.yaml`),
		[]byte("name: Zone A\nroomid: 6403\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(zoneDir, `6403.yaml`),
		[]byte("roomid: 6403\nzone: Zone A\ntitle: Test Room\ndescription: A bare test room.\n"), 0o644))

	require.NoError(t, loadAllRoomZones())
	require.Equal(t, 6404, GetNextRoomId(), `NextRoomId must sit above the highest room on disk`)
}

// (c) The allocator never returns an id that already exists, whatever the
// counter says.
func TestNewRoom_SkipsExistingRoomIds(t *testing.T) {
	useUnwritableNextRoomId(t, 6403)
	freshRoomManager(t)
	roomManager.setCachedFilePath(6403, `zone_a/6403.yaml`)
	roomManager.setCachedFilePath(6404, `zone_a/6404.yaml`)
	roomManager.rooms[6405] = &Room{RoomId: 6405}

	r := NewRoom(`Zone A`)
	require.Equal(t, 6406, r.RoomId)
	require.Equal(t, 6407, GetNextRoomId())

	r2 := NewRoom(`Zone A`)
	require.Equal(t, 6407, r2.RoomId)
}
