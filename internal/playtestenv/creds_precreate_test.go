package playtestenv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// creds.json pre-creation.
//
// The container runs as root and writes creds.json into the control bind with
// mode 0600. On Linux that leaves the artifact owned by root, and the
// unprivileged host user that started the run cannot read it, so every
// profile-based run failed at the point it read creds.json. Docker Desktop
// synthesizes host ownership for bind mounts, so this only ever showed up on
// Linux CI.
//
// The fix is to create the file host-side first and let the container's write
// truncate it in place, which leaves the owner alone.
// ---------------------------------------------------------------------------

func TestPrecreateCredsFile_CreatesOwnerOnlyEmptyFile(t *testing.T) {
	dir := t.TempDir()

	path, err := precreateCredsFile(dir)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, credsFileName), path)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Zero(t, info.Size(), "pre-created creds file must start empty")

	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"creds.json holds plaintext passwords and must stay owner-only")
	}
}

// The property the whole fix rests on: writing to an existing file truncates
// it in place rather than replacing the inode, so the owner and mode survive.
// If this ever stops holding — most likely because writeCredsFile in
// internal/playtestprofiles is changed to an atomic write-temp-then-rename —
// the container's write would revert the file to root ownership and the
// permission-denied failure returns.
func TestPrecreateCredsFile_LaterWritePreservesModeAndInode(t *testing.T) {
	dir := t.TempDir()

	path, err := precreateCredsFile(dir)
	require.NoError(t, err)

	before, err := os.Stat(path)
	require.NoError(t, err)

	// Stand in for the containerized server: same call writeCredsFile makes.
	payload := []byte(`{"run_id":"abc","players":[]}`)
	require.NoError(t, os.WriteFile(path, payload, 0o600))

	after, err := os.Stat(path)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, payload, got)

	if runtime.GOOS != "windows" {
		require.Equal(t, before.Mode().Perm(), after.Mode().Perm(),
			"a later write must not change the file mode")
		require.True(t, os.SameFile(before, after),
			"a later write must reuse the same inode, or ownership reverts to the container user")
	}
}

// Start may be retried against a control dir that already holds a creds file
// from an earlier attempt; the stale contents must not survive.
func TestPrecreateCredsFile_TruncatesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credsFileName)
	require.NoError(t, os.WriteFile(path, []byte(`{"run_id":"stale"}`), 0o600))

	got, err := precreateCredsFile(dir)
	require.NoError(t, err)
	require.Equal(t, path, got)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Zero(t, info.Size(), "stale creds contents must not survive a retry")
}

func TestPrecreateCredsFile_MissingDirIsAnError(t *testing.T) {
	_, err := precreateCredsFile(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
}
