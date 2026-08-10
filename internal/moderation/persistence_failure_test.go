package moderation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.3 / finding 7 — moderation state must not diverge from disk.
//
// Every mutator here writes the in-memory map first and then saves. When the
// save failed, memory kept the change and disk did not, so:
//
//   - a ban looked applied, kicked the player, and was gone after a restart
//   - an unban looked applied and the player was banned again after a restart
//
// The save error was already returned; nothing rolled back, and the ban/unban
// commands discarded it. These tests pin the rollback half.
// ---------------------------------------------------------------------------

// unwritableDir returns a path that cannot be created as a directory, because
// its parent is a regular file. Portable: this fails on Windows too, where
// chmod-based tricks do not.
func unwritableDir(t *testing.T) string {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))
	return filepath.Join(blocker, "moderation")
}

func TestBanAccount_FailedSaveDoesNotLeaveTheBanInMemory(t *testing.T) {
	resetForTest()
	restore := SetDataDirForTest(unwritableDir(t))
	defer restore()

	err := BanAccount("villain", "griefing", "admin")

	require.Error(t, err, "an unwritable store must surface the failure")

	_, banned := IsAccountBanned("villain")
	assert.False(t, banned,
		"memory must not report a ban that never reached disk — it would vanish on restart")
}

func TestUnban_FailedSaveKeepsThePlayerBanned(t *testing.T) {
	resetForTest()

	// Establish a real ban in a working store first.
	good := t.TempDir()
	restoreGood := SetDataDirForTest(good)
	require.NoError(t, BanAccount("villain", "griefing", "admin"))
	restoreGood()

	// Now the store breaks, and the unban cannot be persisted.
	restoreBad := SetDataDirForTest(unwritableDir(t))
	defer restoreBad()

	err := Unban("villain")

	require.Error(t, err)
	_, banned := IsAccountBanned("villain")
	assert.True(t, banned,
		"an unban that did not persist must not appear applied — the ban returns on restart")
}

func TestBanIP_FailedSaveDoesNotLeaveTheBanInMemory(t *testing.T) {
	resetForTest()
	restore := SetDataDirForTest(unwritableDir(t))
	defer restore()

	err := BanIP("203.0.113.7", "spam", "admin")

	require.Error(t, err)
	_, banned := IsIPBanned("203.0.113.7")
	assert.False(t, banned, "memory must not diverge from disk")
}

func TestUnbanIP_FailedSaveKeepsTheIPBanned(t *testing.T) {
	resetForTest()

	good := t.TempDir()
	restoreGood := SetDataDirForTest(good)
	require.NoError(t, BanIP("203.0.113.7", "spam", "admin"))
	restoreGood()

	restoreBad := SetDataDirForTest(unwritableDir(t))
	defer restoreBad()

	err := UnbanIP("203.0.113.7")

	require.Error(t, err)
	_, banned := IsIPBanned("203.0.113.7")
	assert.True(t, banned)
}

// The happy path must still work, so the rollback cannot be a blanket refusal.
func TestBanAndUnban_SucceedAndPersistWhenTheStoreIsWritable(t *testing.T) {
	resetForTest()
	dir := t.TempDir()
	restore := SetDataDirForTest(dir)
	defer restore()

	require.NoError(t, BanAccount("villain", "griefing", "admin"))
	reason, banned := IsAccountBanned("villain")
	require.True(t, banned)
	assert.Equal(t, "griefing", reason)

	// The file is really on disk, written through the durable path.
	_, err := os.Stat(filepath.Join(dir, "bans.yaml"))
	require.NoError(t, err)
	_, tmpErr := os.Stat(filepath.Join(dir, "bans.yaml.new"))
	assert.True(t, os.IsNotExist(tmpErr), "temp file must not survive a successful save")

	require.NoError(t, Unban("villain"))
	_, banned = IsAccountBanned("villain")
	assert.False(t, banned)
}

// Re-banning after a failed ban must still work: the rollback must leave the
// map genuinely clean, not in a half-state.
func TestBanAccount_RollbackLeavesTheStoreReusable(t *testing.T) {
	resetForTest()

	restoreBad := SetDataDirForTest(unwritableDir(t))
	require.Error(t, BanAccount("villain", "griefing", "admin"))
	restoreBad()

	dir := t.TempDir()
	restore := SetDataDirForTest(dir)
	defer restore()

	require.NoError(t, BanAccount("villain", "griefing", "admin"))
	_, banned := IsAccountBanned("villain")
	assert.True(t, banned, "a retry after a failed save must succeed cleanly")
}
