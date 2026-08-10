package util

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.1 — the living-state persistence contract.
//
// Living state is the world's accumulated, unreproducible data: mob instance
// progression, shop economies, guilds, bans and petitions. Losing it is not
// recoverable by rebooting, unlike authored content which can be reloaded from
// the repo.
//
// The contract has three parts, and the read half is the one the review kept
// finding broken (findings 5, 6, 7, 15): every store collapsed "file is
// corrupt" into "file is absent", and absent means "seed fresh defaults". One
// bad byte silently reset a shop's economy or dropped a ban.
// ---------------------------------------------------------------------------

// --- write half: SafeSave must be crash-durable, not merely atomic ----------

func TestSafeSave_WritesContentAndLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	require.NoError(t, SafeSave(path, []byte("hello")))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "the .new temp file must not survive a successful save")
	assert.Equal(t, "state.yaml", entries[0].Name())
}

func TestSafeSave_OverwritesAtomicallyWithoutTruncating(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")
	require.NoError(t, os.WriteFile(path, []byte("old contents"), 0o644))

	require.NoError(t, SafeSave(path, []byte("new")))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
}

// A failed save must not leave a .new file behind to confuse the next reader
// or accumulate on disk.
func TestSafeSave_CleansUpTempOnFailure(t *testing.T) {
	dir := t.TempDir()
	// A directory where the target name is itself a directory: the rename
	// cannot succeed, so the temp file must be cleaned up.
	path := filepath.Join(dir, "state.yaml")
	require.NoError(t, os.Mkdir(path, 0o755))

	err := SafeSave(path, []byte("data"))
	require.Error(t, err, "renaming over a directory must fail")

	_, statErr := os.Stat(path + ".new")
	assert.True(t, os.IsNotExist(statErr), "temp file must not survive a failed save")
}

// Data files have no business being executable.
func TestSafeSave_UsesNonExecutableMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not meaningful on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	require.NoError(t, SafeSave(path, []byte("x")))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

// --- read half: absent and corrupt must be distinguishable ------------------

func TestReadLivingState_ReturnsContentWhenPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")
	require.NoError(t, os.WriteFile(path, []byte("payload"), 0o644))

	data, err := ReadLivingState(path)
	require.NoError(t, err)
	assert.Equal(t, "payload", string(data))
}

// The whole point of the chunk: a missing file is normal (first run) and must
// be reported differently from a file that exists but cannot be read.
func TestReadLivingState_AbsentIsItsOwnError(t *testing.T) {
	data, err := ReadLivingState(filepath.Join(t.TempDir(), "nope.yaml"))

	assert.Nil(t, data)
	assert.True(t, errors.Is(err, ErrStateAbsent), "missing file must report ErrStateAbsent")
	assert.False(t, errors.Is(err, ErrStateCorrupt), "missing must not read as corrupt")
}

func TestReadLivingState_UnreadableIsCorruptNotAbsent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based unreadability is not reliable on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")
	require.NoError(t, os.WriteFile(path, []byte("payload"), 0o644))
	require.NoError(t, os.Chmod(path, 0o000))
	defer os.Chmod(path, 0o644)

	if os.Geteuid() == 0 {
		t.Skip("running as root defeats permission checks")
	}

	data, err := ReadLivingState(path)

	assert.Nil(t, data)
	assert.True(t, errors.Is(err, ErrStateCorrupt), "unreadable existing file must be ErrStateCorrupt")
	assert.False(t, errors.Is(err, ErrStateAbsent), "an existing file must never read as absent")
}

// --- quarantine -------------------------------------------------------------

func TestQuarantineCorrupt_MovesFileAsideAndPreservesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")
	require.NoError(t, os.WriteFile(path, []byte("corrupt payload"), 0o644))

	dest, err := QuarantineCorrupt(path)
	require.NoError(t, err)

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "the corrupt file must no longer be readable as live state")

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "corrupt payload", string(got),
		"quarantine preserves the bytes for inspection; it never deletes")

	assert.True(t, strings.HasPrefix(filepath.Base(dest), "state.yaml.corrupt-"),
		"quarantine name must be traceable to the original, got %q", filepath.Base(dest))
}

// Two corruptions of the same file must not clobber each other's evidence.
func TestQuarantineCorrupt_DoesNotOverwriteAnEarlierQuarantine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	require.NoError(t, os.WriteFile(path, []byte("first"), 0o644))
	first, err := QuarantineCorrupt(path)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("second"), 0o644))
	second, err := QuarantineCorrupt(path)
	require.NoError(t, err)

	require.NotEqual(t, first, second, "second quarantine must not reuse the first name")

	a, err := os.ReadFile(first)
	require.NoError(t, err)
	b, err := os.ReadFile(second)
	require.NoError(t, err)
	assert.Equal(t, "first", string(a))
	assert.Equal(t, "second", string(b))
}

func TestQuarantineCorrupt_MissingFileIsAnError(t *testing.T) {
	_, err := QuarantineCorrupt(filepath.Join(t.TempDir(), "nope.yaml"))
	assert.Error(t, err)
}

// A store that quarantines and then seeds defaults must be able to save over
// the now-absent path immediately.
func TestQuarantineThenSave_IsTheDocumentedRecoveryPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")
	require.NoError(t, os.WriteFile(path, []byte("bad"), 0o644))

	_, err := QuarantineCorrupt(path)
	require.NoError(t, err)

	_, err = ReadLivingState(path)
	require.True(t, errors.Is(err, ErrStateAbsent),
		"after quarantine the path reads as absent, so the store seeds defaults")

	require.NoError(t, SafeSave(path, []byte("fresh defaults")))
	got, err := ReadLivingState(path)
	require.NoError(t, err)
	assert.Equal(t, "fresh defaults", string(got))
}
