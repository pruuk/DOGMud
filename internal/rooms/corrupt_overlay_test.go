package rooms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.4 / finding 15 — a corrupt room instance overlay must be rejected
// whole, not applied halfway.
//
// LoadRoomInstance unmarshalled the instance file directly ONTO the loaded
// template. yaml.Unmarshal applies fields as it walks the document, so a file
// that is valid for a while and then breaks leaves the room already mutated
// when the error is returned. The old code logged a Warn and carried on with
// exactly that half-applied room, handing players a template/runtime hybrid:
// some fields from the template, some from a damaged save.
//
// The fix is to build the overlay on a scratch copy and only adopt it if the
// whole document parsed.
// ---------------------------------------------------------------------------

func quarantinedOverlays(t *testing.T, path string) []string {
	t.Helper()
	m, err := filepath.Glob(path + ".corrupt-*")
	require.NoError(t, err)
	return m
}

// A valid overlay must still be applied — the control leg.
func TestLoadRoomInstance_ValidOverlayIsApplied(t *testing.T) {
	tempDir := useTempDataFiles(t, true)
	instancePath := seedTemplateRoom(t, tempDir, 100)

	require.NoError(t, os.WriteFile(instancePath, []byte("roomid: 100\ngold: 250\n"), 0o644))

	room := LoadRoomInstance(100)

	require.NotNil(t, room)
	assert.Equal(t, 250, room.Gold, "a valid overlay must be applied")
	assert.Empty(t, quarantinedOverlays(t, instancePath))
}

// The finding. The corrupt file below sets gold before it breaks, so a
// half-applied overlay is directly observable.
func TestLoadRoomInstance_CorruptOverlayIsRejectedWholeNotAppliedPartially(t *testing.T) {
	tempDir := useTempDataFiles(t, true)
	instancePath := seedTemplateRoom(t, tempDir, 100)

	corrupt := "roomid: 100\ngold: 999\nnouns:\n  broken: [unclosed\n"
	require.NoError(t, os.WriteFile(instancePath, []byte(corrupt), 0o644))

	room := LoadRoomInstance(100)

	require.NotNil(t, room, "the room must still load, from pure template state")
	assert.Equal(t, 0, room.Gold,
		"gold from a corrupt overlay must NOT survive — that is the template/runtime hybrid")
	assert.Equal(t, "Careful Save Room", room.Title, "template fields must be intact")

	q := quarantinedOverlays(t, instancePath)
	require.Len(t, q, 1, "the corrupt overlay must be quarantined")
	got, err := os.ReadFile(q[0])
	require.NoError(t, err)
	assert.Equal(t, corrupt, string(got), "quarantine preserves the original bytes")

	_, statErr := os.Stat(instancePath)
	assert.True(t, os.IsNotExist(statErr),
		"the corrupt overlay must be moved aside so the next save cannot overwrite the evidence")
}

// After rejection the room must be saveable again, or it can never persist.
func TestLoadRoomInstance_QuarantinedOverlayLeavesTheRoomSaveable(t *testing.T) {
	tempDir := useTempDataFiles(t, true)
	instancePath := seedTemplateRoom(t, tempDir, 100)

	require.NoError(t, os.WriteFile(instancePath, []byte("roomid: 100\ngold: [broken\n"), 0o644))

	room := LoadRoomInstance(100)
	require.NotNil(t, room)
	require.Len(t, quarantinedOverlays(t, instancePath), 1)

	room.Gold = 42
	require.NoError(t, SaveRoomInstance(*room), "the quarantined path must be writable again")

	_, err := os.Stat(instancePath)
	require.NoError(t, err, "a fresh instance file must exist after the save")
	assert.Len(t, quarantinedOverlays(t, instancePath), 1,
		"the recovery must not create a second quarantine")
}

// A missing overlay is the ordinary case for a room nobody has changed.
func TestLoadRoomInstance_AbsentOverlayIsSilent(t *testing.T) {
	tempDir := useTempDataFiles(t, true)
	instancePath := seedTemplateRoom(t, tempDir, 100)

	room := LoadRoomInstance(100)

	require.NotNil(t, room)
	assert.Equal(t, "Careful Save Room", room.Title)
	assert.Empty(t, quarantinedOverlays(t, instancePath),
		"no overlay file is not a corruption")
}
