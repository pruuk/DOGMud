package rooms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/stretchr/testify/require"
)

// Owner ruling 18 (2026-11-26): a named room exit is at most
// exit.MaxNameLetters letters, so the room loader refuses a longer one at
// boot, naming the room and the exit. Compass words are exempt.
func TestRoomValidate_RefusesAnExitNameLongerThanAButton(t *testing.T) {
	r := &Room{
		RoomId:      5243,
		Title:       "Tower Top",
		Description: "A broken crown.",
		Exits: map[string]exit.RoomExit{
			"west":      {RoomId: 5242},
			"northeast": {RoomId: 1},
			"shrine":    {RoomId: 5346},
			"outlook":   {RoomId: 5365}, // 7 letters
		},
	}
	err := r.Validate()
	require.Error(t, err, "a seven-letter named exit must be refused")
	require.Contains(t, err.Error(), "5243")
	require.Contains(t, err.Error(), `"outlook"`)

	delete(r.Exits, "outlook")
	r.Exits["ledge"] = exit.RoomExit{RoomId: 5365}
	require.NoError(t, r.Validate(), "six letters or fewer, and compass words, pass")
}

// The instance's way back out is the "return" exit (was "return portal"),
// short enough for a deck button (owner ruling 18, 2026-11-26).
func TestInstanceReturnExitName_FitsAButton(t *testing.T) {
	require.Equal(t, "return", instanceReturnExitName, "the owner ruled return")
	require.NoError(t, exit.CheckName(instanceReturnExitName))
}

// The builder's disk boundary refuses the same name, so no builder save
// writes a room the next boot would refuse.
func TestSaveRoomTemplate_RefusesAnExitNameLongerThanAButton(t *testing.T) {
	tempDir := useTempDataFiles(t, false)
	r := Room{
		RoomId:      777001,
		Zone:        "Test Zone",
		Title:       "Test Room",
		Description: "A test room.",
		Exits:       map[string]exit.RoomExit{"outlook": {RoomId: 2}},
	}
	err := SaveRoomTemplate(r)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"outlook"`)
	_, statErr := os.Stat(filepath.Join(tempDir, "rooms", "test_zone", "777001.yaml"))
	require.True(t, os.IsNotExist(statErr), "the refused room must not reach disk")
}
