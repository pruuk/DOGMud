package rooms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.7 / finding 35 — a failed autosave must not report success.
//
// SaveAllRooms counted failures into a log line and then `return nil`
// unconditionally, so its caller could not distinguish a clean save from one
// where every room failed. The autosave hook then broadcast "Done." to every
// connected player either way.
// ---------------------------------------------------------------------------

func TestSaveAllRooms_ReturnsNilWhenEverythingSaves(t *testing.T) {
	tempDir := useTempDataFiles(t, true)
	seedTemplateRoom(t, tempDir, 100)

	room := LoadRoomInstance(100)
	require.NotNil(t, room)
	room.Gold = 10
	require.NoError(t, addRoomToMemory(room))
	t.Cleanup(func() { removeRoomFromMemory(room) })

	assert.NoError(t, SaveAllRooms(), "a clean save must report success")
}

// The finding: an unwritable store must surface as an error, not a nil return
// with the failure buried in a log line.
func TestSaveAllRooms_ReportsFailureInsteadOfReturningNil(t *testing.T) {
	tempDir := useTempDataFiles(t, true)
	seedTemplateRoom(t, tempDir, 100)

	room := LoadRoomInstance(100)
	require.NotNil(t, room)
	room.Gold = 10 // give it something instance-specific so it actually writes
	require.NoError(t, addRoomToMemory(room))
	t.Cleanup(func() { removeRoomFromMemory(room) })

	// Make the instance directory unwritable by replacing it with a file, so
	// the save cannot create its target. Portable, unlike chmod on Windows.
	instDir := filepath.Join(tempDir, "rooms.instances", "test_zone")
	require.NoError(t, os.RemoveAll(instDir))
	require.NoError(t, os.WriteFile(instDir, []byte("x"), 0o644))

	err := SaveAllRooms()

	require.Error(t, err, "SaveAllRooms must report that rooms failed to save")
	assert.Contains(t, err.Error(), "failed to save",
		"the error should say how many rooms failed, so the log is actionable")
}
