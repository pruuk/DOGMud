package guilds

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.3 / finding 7 — guild membership must not diverge from disk.
//
// Every mutator published to the in-memory registry first and saved after, so a
// failed save left a player who "joined" a guild in memory only. On restart
// they were not in it. Create() already rolled back on save failure; the other
// mutators did not.
// ---------------------------------------------------------------------------

// unwritableDir returns a path that cannot be created as a directory because
// its parent is a regular file. Portable, unlike chmod tricks on Windows.
func unwritableDir(t *testing.T) string {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))
	return filepath.Join(blocker, "guilds")
}

// newTestGuild creates a guild in a writable store and returns it plus a
// restore func for the data dir.
func newTestGuild(t *testing.T) *Guild {
	t.Helper()
	restore := SetDataDirForTest(t.TempDir())
	t.Cleanup(restore)
	g, err := Create("TST", "Testers", 1, "Leader")
	require.NoError(t, err)
	require.NotNil(t, g)
	t.Cleanup(func() { Delete(g.Tag) })
	return g
}

func TestAddMember_FailedSaveDoesNotLeaveThePlayerInTheGuild(t *testing.T) {
	g := newTestGuild(t)

	// Break the store, then try to add a member.
	restoreBad := SetDataDirForTest(unwritableDir(t))
	defer restoreBad()

	err := AddMember(g.Tag, 42, "Newbie")

	require.Error(t, err, "an unwritable store must surface the failure")

	assert.False(t, g.IsMember(42),
		"membership must not exist in memory when it never reached disk")
	_, inGuild := GetByUser(42)
	assert.False(t, inGuild,
		"the byUser index must roll back too, or the player is locked out of joining any guild")
}

func TestRemoveMember_FailedSaveKeepsThePlayerInTheGuild(t *testing.T) {
	g := newTestGuild(t)
	require.NoError(t, AddMember(g.Tag, 42, "Newbie"))
	require.True(t, g.IsMember(42))

	restoreBad := SetDataDirForTest(unwritableDir(t))
	defer restoreBad()

	err := RemoveMember(g.Tag, 42)

	require.Error(t, err)
	assert.True(t, g.IsMember(42),
		"a removal that did not persist must not appear applied — they return on restart")
	_, inGuild := GetByUser(42)
	assert.True(t, inGuild, "the byUser index must match the membership list")
}

func TestSetRank_FailedSaveKeepsThePreviousRank(t *testing.T) {
	g := newTestGuild(t)
	require.NoError(t, AddMember(g.Tag, 42, "Newbie"))

	restoreBad := SetDataDirForTest(unwritableDir(t))
	defer restoreBad()

	err := SetRank(g.Tag, 42, RankOfficer)

	require.Error(t, err)
	for _, m := range g.Members {
		if m.UserId == 42 {
			assert.Equal(t, RankMember, m.Rank,
				"a promotion that did not persist must not appear applied")
		}
	}
}

// Control legs: the happy paths must still work, so rollback cannot be a
// blanket refusal.
func TestGuildMutations_SucceedAndPersistWhenTheStoreIsWritable(t *testing.T) {
	dir := t.TempDir()
	restore := SetDataDirForTest(dir)
	defer restore()

	g, err := Create("OK1", "Fine", 1, "Leader")
	require.NoError(t, err)
	defer Delete(g.Tag)

	require.NoError(t, AddMember(g.Tag, 42, "Newbie"))
	assert.True(t, g.IsMember(42))

	require.NoError(t, SetRank(g.Tag, 42, RankOfficer))

	// Written through the durable path: no temp file left behind.
	_, tmpErr := os.Stat(filepath.Join(dir, "ok1.yaml.new"))
	assert.True(t, os.IsNotExist(tmpErr), "temp file must not survive a successful save")
	_, statErr := os.Stat(filepath.Join(dir, "ok1.yaml"))
	require.NoError(t, statErr, "the guild file must exist on disk")

	require.NoError(t, RemoveMember(g.Tag, 42))
	assert.False(t, g.IsMember(42))
}

// A retry after a failed add must work cleanly rather than hitting "already in
// a guild" from a half-applied byUser entry.
func TestAddMember_RollbackLeavesThePlayerAbleToJoin(t *testing.T) {
	g := newTestGuild(t)

	restoreBad := SetDataDirForTest(unwritableDir(t))
	require.Error(t, AddMember(g.Tag, 42, "Newbie"))
	restoreBad()

	require.NoError(t, AddMember(g.Tag, 42, "Newbie"),
		"a retry after a failed save must not be blocked by stale index state")
	assert.True(t, g.IsMember(42))
}
