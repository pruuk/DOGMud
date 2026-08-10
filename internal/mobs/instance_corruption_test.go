package mobs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.2 / finding 5 — mob instance persistence must follow the living-state
// contract (internal/util/livingstate.go).
//
// Two defects, one on each side:
//
//   - The save used a bare os.WriteFile, so an interrupted write truncated the
//     file in place.
//   - The load returned nil for BOTH "no file" and "file is corrupt", and nil
//     means "fresh spawn from template". So a truncated file silently discarded
//     the mob's stats, skills, mutations, inventory, gold and planner state —
//     and the next save then overwrote the evidence.
//
// A mob instance file is living state: it is the only record of what that mob
// became. It cannot be regenerated from the repo.
// ---------------------------------------------------------------------------

// quarantineGlob returns any quarantined siblings of an instance path.
func quarantineGlob(t *testing.T, path string) []string {
	t.Helper()
	matches, err := filepath.Glob(path + ".corrupt-*")
	require.NoError(t, err)
	return matches
}

// cleanInstancePath removes an instance file and any quarantined siblings so
// tests do not leak state into each other.
func cleanInstancePath(t *testing.T, path string) {
	t.Helper()
	_ = os.Remove(path)
	for _, m := range quarantineGlob(t, path) {
		_ = os.Remove(m)
	}
}

// A mob with progression must round-trip. This is the control leg: it proves
// the corruption tests below are exercising a path that otherwise works.
func TestLoadMobInstance_RoundTripsThroughTheDurableSavePath(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()
	withMobProgressionEnabled(t)

	mob := NewMobById(1, 100)
	require.NotNil(t, mob)
	path := instancePath(mob.MobId, mob.Zone, mob.Character.Name, mob.HomeRoomId)
	cleanInstancePath(t, path)
	t.Cleanup(func() { cleanInstancePath(t, path) })

	mob.Character.Stats.Strength.Training = 7
	require.NoError(t, SaveMobInstance(mob))

	// The durable save writes through a temp file; it must not survive.
	_, tmpErr := os.Stat(path + ".new")
	assert.True(t, os.IsNotExist(tmpErr), "the .new temp file must not survive a successful save")

	loaded := LoadMobInstance(mob.MobId, mob.Zone, mob.Character.Name, mob.HomeRoomId)
	require.NotNil(t, loaded, "a saved instance must load back")
	assert.Equal(t, 7, loaded.StrengthTraining)
}

// Absent is the normal first-spawn case: nil, quietly, and nothing quarantined.
func TestLoadMobInstance_AbsentReturnsNilAndQuarantinesNothing(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	path := instancePath(4242, "nowhere", "ghost", 4242)
	cleanInstancePath(t, path)

	loaded := LoadMobInstance(4242, "nowhere", "ghost", 4242)

	assert.Nil(t, loaded, "no file means fresh spawn")
	assert.Empty(t, quarantineGlob(t, path), "an absent file is not a corruption; nothing to quarantine")
}

// The finding's core case. A malformed file must NOT read as a fresh spawn
// without a trace: the bytes have to be preserved and the operator told.
func TestLoadMobInstance_MalformedFileIsQuarantinedNotSilentlyDiscarded(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()
	withMobProgressionEnabled(t)

	mob := NewMobById(1, 100)
	require.NotNil(t, mob)
	path := instancePath(mob.MobId, mob.Zone, mob.Character.Name, mob.HomeRoomId)
	cleanInstancePath(t, path)
	t.Cleanup(func() { cleanInstancePath(t, path) })

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	// Truncated mid-map, the shape a torn write actually produces.
	corrupt := "strength_training: 12\nskills:\n  brawling: 4\n  \x00\x00nope: [unclosed\n"
	require.NoError(t, os.WriteFile(path, []byte(corrupt), 0o644))

	loaded := LoadMobInstance(mob.MobId, mob.Zone, mob.Character.Name, mob.HomeRoomId)

	assert.Nil(t, loaded, "a corrupt file cannot yield progression data")

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr),
		"the corrupt file must be moved aside so the next save cannot overwrite the evidence")

	q := quarantineGlob(t, path)
	require.Len(t, q, 1, "exactly one quarantine file expected")
	got, err := os.ReadFile(q[0])
	require.NoError(t, err)
	assert.Equal(t, corrupt, string(got), "quarantine preserves the original bytes verbatim")
	assert.True(t, strings.Contains(filepath.Base(q[0]), ".corrupt-"),
		"quarantine name must be recognisable, got %q", filepath.Base(q[0]))
}

// Unreadable is a different failure from unparseable, and must reach the same
// non-destructive handling rather than being mistaken for absent.
func TestLoadMobInstance_UnreadableFileIsTreatedAsCorruptNotAbsent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based unreadability is not reliable on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root defeats permission checks")
	}
	cleanup := seedRegistry()
	defer cleanup()

	path := instancePath(7, "unreadzone", "lockedmob", 77)
	cleanInstancePath(t, path)
	t.Cleanup(func() {
		_ = os.Chmod(path, 0o644)
		cleanInstancePath(t, path)
	})

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("strength_training: 3\n"), 0o644))
	require.NoError(t, os.Chmod(path, 0o000))

	loaded := LoadMobInstance(7, "unreadzone", "lockedmob", 77)

	assert.Nil(t, loaded)
	assert.Len(t, quarantineGlob(t, path), 1,
		"an unreadable existing file must be quarantined, not mistaken for a fresh spawn")
}

// The documented recovery path end to end: corrupt load quarantines, the mob
// respawns from template, and the next save writes a clean file that loads.
func TestLoadMobInstance_QuarantineThenSaveRecoversCleanly(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()
	withMobProgressionEnabled(t)

	mob := NewMobById(1, 100)
	require.NotNil(t, mob)
	path := instancePath(mob.MobId, mob.Zone, mob.Character.Name, mob.HomeRoomId)
	cleanInstancePath(t, path)
	t.Cleanup(func() { cleanInstancePath(t, path) })

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("this: [is not: valid yaml\n"), 0o644))

	require.Nil(t, LoadMobInstance(mob.MobId, mob.Zone, mob.Character.Name, mob.HomeRoomId))
	require.Len(t, quarantineGlob(t, path), 1)

	mob.Character.Stats.Vitality.Training = 5
	require.NoError(t, SaveMobInstance(mob), "a quarantined path must be writable again")

	reloaded := LoadMobInstance(mob.MobId, mob.Zone, mob.Character.Name, mob.HomeRoomId)
	require.NotNil(t, reloaded)
	assert.Equal(t, 5, reloaded.VitalityTraining)
	assert.Len(t, quarantineGlob(t, path), 1, "the recovery must not create a second quarantine")
}
