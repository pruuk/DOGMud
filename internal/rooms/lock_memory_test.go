package rooms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const lockMemoryRoomId = 100

// lockMemoryRoom writes a template on disk for room 100 with a locked north
// exit and a locked chest, and one for room 101 beyond it, in a fresh room
// manager, with the shipped timing pinned so a relock interval is a real span
// of rounds. It returns room 100's template path.
func lockMemoryRoom(t *testing.T) string {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	prevRound := util.GetRoundCount()
	util.SetRoundCountForTest(uint64(util.RoundCountMinimum) + 1000)
	t.Cleanup(func() { util.SetRoundCountForTest(prevRound) })

	tempDir := useTempDataFiles(t, false)
	t.Cleanup(SeedRoomsForTest(map[int]*Room{}, map[string]*ZoneConfig{}))
	t.Cleanup(func() { InvalidateTemplateCache(lockMemoryRoomId) })

	tpl := &Room{
		RoomId:      lockMemoryRoomId,
		Zone:        "test_zone",
		Title:       "Lock Room",
		Description: "A room whose locks outlive an unload.",
		Exits: map[string]exit.RoomExit{
			"north": {RoomId: 101, Lock: gamelock.Lock{Difficulty: 6}},
		},
		Containers: map[string]Container{"chest": {Lock: gamelock.Lock{Difficulty: 6}}},
	}
	data, err := yaml.Marshal(tpl)
	require.NoError(t, err)
	templatePath := filepath.Join(tempDir, "rooms", "test_zone", "100.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(templatePath), 0755))
	require.NoError(t, os.WriteFile(templatePath, data, 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "rooms.instances", "test_zone"), 0755))
	roomManager.roomIdToFileCache[lockMemoryRoomId] = "test_zone/100.yaml"

	beyond, err := yaml.Marshal(&Room{RoomId: 101, Zone: "test_zone", Title: "Beyond", Description: "The room past the lock."})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "rooms", "test_zone", "101.yaml"), beyond, 0644))
	roomManager.roomIdToFileCache[101] = "test_zone/101.yaml"
	t.Cleanup(func() { InvalidateTemplateCache(101) })
	return templatePath
}

// rotatedLockRoom loads room 100 and relocks its north exit once, so the exit
// lock is at RotationSeed 1.
func rotatedLockRoom(t *testing.T) *Room {
	t.Helper()
	room := LoadRoom(lockMemoryRoomId)
	require.NotNil(t, room)
	room.SetExitLock("north", false)
	room.SetExitLock("north", true)
	require.Equal(t, uint64(1), room.Exits["north"].Lock.RotationSeed, "premise: the relock rotated the lock")
	return room
}

// A builder save marshals the live room. Its template file holds every exit
// lock at rotation 0 (no template may set an exit's rotationseed), and the
// live room, which shares the Exits map, keeps its rotation; so does the room
// SaveRoomTemplate puts in memory in its place (addRoomToMemory with force,
// through ClearRoomCache and restoreLocks).
func TestSaveRoomTemplate_KeepsExitRotationOutOfTheFileAndInPlay(t *testing.T) {
	templatePath := lockMemoryRoom(t)
	room := rotatedLockRoom(t)

	require.NoError(t, SaveRoomTemplate(*room))

	raw, err := os.ReadFile(templatePath)
	require.NoError(t, err)
	var saved struct {
		Exits map[string]struct {
			Lock map[string]any `yaml:"lock"`
		} `yaml:"exits"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &saved))
	require.Contains(t, saved.Exits, "north", "premise: the saved template has the exit")
	require.NotContains(t, saved.Exits["north"].Lock, "rotationseed", "the template file carries the exit's play rotation:\n%s", raw)

	require.Equal(t, uint64(1), room.Exits["north"].Lock.RotationSeed, "saving zeroed the live room's exit rotation")
	inPlay := LoadRoom(lockMemoryRoomId)
	require.NotSame(t, room, inPlay, "premise: SaveRoomTemplate put a new room in memory")
	require.Equal(t, uint64(1), inPlay.Exits["north"].Lock.RotationSeed, "the room SaveRoomTemplate put in memory lost the exit's rotation")

	// A save of a room read fresh from the template file (as ConnectRoom
	// does) shares nothing with the live room; the rotation reaches the new
	// room only through ClearRoomCache's record and restoreLocks.
	fresh := LoadRoomTemplate(lockMemoryRoomId)
	require.NotNil(t, fresh)
	require.Equal(t, uint64(0), fresh.Exits["north"].Lock.RotationSeed, "premise: the template file is at rotation 0")
	require.NoError(t, SaveRoomTemplate(*fresh))
	require.Equal(t, uint64(1), LoadRoom(lockMemoryRoomId).Exits["north"].Lock.RotationSeed, "a template save put the exit lock back at rotation 0")
}

// ConnectRoom builds its room from the template file and puts it in memory in
// place of the live one; the exit lock keeps its rotation.
func TestConnectRoom_KeepsExitRotation(t *testing.T) {
	lockMemoryRoom(t)
	rotatedLockRoom(t)

	require.NoError(t, ConnectRoom(lockMemoryRoomId, 101, "east"))

	inPlay := LoadRoom(lockMemoryRoomId)
	require.Contains(t, inPlay.Exits, "east", "premise: the exit was built")
	require.Equal(t, uint64(1), inPlay.Exits["north"].Lock.RotationSeed, "connecting a room put its exit lock back at the template's rotation")
}

// Room.Exits is instance:"skip", so a reloaded room's exit locks come from
// the template at RotationSeed 0. The lock state remembered at the unload
// keeps the rotation, so a combination retired by a relock stays retired.
func TestUnloadAndReload_KeepsTheExitsRotation(t *testing.T) {
	lockMemoryRoom(t)
	const lockId = `100-north`
	t.Cleanup(util.RebootLocksUntilRotatedForTest(6, lockId))

	room := LoadRoom(lockMemoryRoomId)
	require.NotNil(t, room)
	preLock := room.Exits["north"].Lock.Sequence(lockId)
	room.SetExitLock("north", false)
	room.SetExitLock("north", true)
	require.Equal(t, uint64(1), room.Exits["north"].Lock.RotationSeed, "premise: the relock rotated the lock")

	removeRoomFromMemory(room)
	require.False(t, IsRoomLoaded(lockMemoryRoomId), "premise: the room unloaded")

	again := LoadRoom(lockMemoryRoomId)
	require.NotNil(t, again)
	require.NotSame(t, room, again, "premise: the room was read again")
	require.Equal(t, uint64(1), again.Exits["north"].Lock.RotationSeed, "the reload put the exit back at the template's rotation")
	require.NotEqual(t, preLock, again.Exits["north"].Lock.Sequence(lockId), "the combination from before the lock works again after a reload")
}

// UnlockedRound is never saved, so an open exit or chest came back locked at
// the rotation it was opened at: a relock with no new combination. The
// remembered state keeps both open across the unload.
func TestUnloadAndReload_KeepsOpenLocksOpen(t *testing.T) {
	lockMemoryRoom(t)

	room := LoadRoom(lockMemoryRoomId)
	require.NotNil(t, room)
	room.SetExitLock("north", false)
	chest := room.Containers["chest"]
	chest.Lock.SetUnlocked()
	room.Containers["chest"] = chest

	removeRoomFromMemory(room)
	again := LoadRoom(lockMemoryRoomId)
	require.NotNil(t, again)
	require.False(t, again.Exits["north"].Lock.IsLocked(), "an open exit came back locked after a reload")
	require.False(t, again.Containers["chest"].Lock.IsLocked(), "an open chest came back locked after a reload")
}

// ExitLockAt reads an exit's lock and never touches disk: live, then
// remembered, then the caller's difficulty at rotation 0.
func TestExitLockAt_LiveThenRememberedThenDifficulty(t *testing.T) {
	lockMemoryRoom(t)
	templateCached := func() bool {
		templateCacheMu.RLock()
		defer templateCacheMu.RUnlock()
		_, ok := templateCache[lockMemoryRoomId]
		return ok
	}

	lock := ExitLockAt(lockMemoryRoomId, "north", 6)
	require.Equal(t, gamelock.Lock{Difficulty: 6}, lock, "a room never loaded is read as the caller's difficulty at rotation 0")
	require.False(t, IsRoomLoaded(lockMemoryRoomId), "ExitLockAt loaded the room")
	require.False(t, templateCached(), "ExitLockAt read the template")

	room := LoadRoom(lockMemoryRoomId)
	room.SetExitLock("north", false)
	room.SetExitLock("north", true)
	require.Equal(t, uint64(1), ExitLockAt(lockMemoryRoomId, "north", 6).RotationSeed, "the live room's lock was not read")

	removeRoomFromMemory(room)
	InvalidateTemplateCache(lockMemoryRoomId) // the unload's save may cache it; ExitLockAt must not
	require.Equal(t, uint64(1), ExitLockAt(lockMemoryRoomId, "north", 6).RotationSeed, "the lock remembered at the unload was not read")
	require.False(t, IsRoomLoaded(lockMemoryRoomId), "ExitLockAt loaded the room")
	require.False(t, templateCached(), "ExitLockAt read the template")
}
