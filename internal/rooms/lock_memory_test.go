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
// exit and a locked chest, in a fresh room manager, with the shipped timing
// pinned so a relock interval is a real span of rounds.
func lockMemoryRoom(t *testing.T) {
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
