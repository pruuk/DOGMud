package dialogue

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.5 / finding 14 — a fixable dialogue file must not be cached as
// "this mob has no dialogue".
//
// Load stored the nil sentinel for three different situations: file absent,
// file unreadable, and file unparseable. Only the first is permanent. Caching
// the other two meant an author who fixed a YAML typo had a mute NPC until the
// whole server was restarted — and the builder workflow is exactly where those
// typos happen.
// ---------------------------------------------------------------------------

// withDialogueDir points DataFiles at a temp tree and clears the caches.
func withDialogueDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := configs.GetFilePathsConfig().DataFiles.String()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": dir}))
	resetCaches()
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prev})
		resetCaches()
	})
	return dir
}

func resetCaches() {
	dialogueCache = map[string]*DialogueFile{}
	nilSentinel = map[string]bool{}
	loadFailureLogged = map[string]string{}
}

func writeDialogue(t *testing.T, dir, zone string, mobId int, body string) string {
	t.Helper()
	p := filepath.Join(dir, "dialogue", zoneNameSanitize(zone), "1234.yaml")
	if mobId != 1234 {
		p = filepath.Join(dir, "dialogue", zoneNameSanitize(zone), "9999.yaml")
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

// The finding: a broken file must be retryable in-process.
func TestLoad_ParseFailureIsNotCachedSoAFixCanBeRetried(t *testing.T) {
	dir := withDialogueDir(t)
	path := writeDialogue(t, dir, "testzone", 1234, "mobid: 1234\npatterns: [unclosed\n")

	require.Nil(t, Load(1234, "testzone"), "a broken file yields no dialogue")

	// The author fixes the typo. No restart.
	require.NoError(t, os.WriteFile(path, []byte("mobid: 1234\ndefaultMood: friendly\n"), 0o644))

	df := Load(1234, "testzone")
	require.NotNil(t, df, "the corrected file must load without restarting the process")
	assert.Equal(t, 1234, df.MobId)
}

// A genuinely absent file SHOULD still be cached — that is the optimisation the
// sentinel exists for, and most mobs never talk.
func TestLoad_AbsentFileIsStillCachedAsNoDialogue(t *testing.T) {
	withDialogueDir(t)

	require.Nil(t, Load(1234, "testzone"))

	assert.True(t, nilSentinel["1234:testzone"],
		"confirmed absence must still be cached, or every silent mob stats the disk each time")
}

// A parse failure must leave no sentinel behind at all.
func TestLoad_ParseFailureLeavesNoSentinel(t *testing.T) {
	dir := withDialogueDir(t)
	writeDialogue(t, dir, "testzone", 1234, "mobid: 1234\nbroken: [\n")

	require.Nil(t, Load(1234, "testzone"))

	assert.False(t, nilSentinel["1234:testzone"],
		"a fixable parse error must not be recorded as 'this mob has no dialogue'")
}

// The error log is throttled so a broken file does not spam every interaction,
// but the throttle must clear once the file is good again.
func TestLoad_FailureLogThrottleClearsAfterASuccessfulLoad(t *testing.T) {
	dir := withDialogueDir(t)
	path := writeDialogue(t, dir, "testzone", 1234, "mobid: 1234\nbroken: [\n")

	require.Nil(t, Load(1234, "testzone"))
	require.NotEmpty(t, loadFailureLogged["1234:testzone"], "the failure should be recorded once")

	require.Nil(t, Load(1234, "testzone"), "still broken, still nil")

	require.NoError(t, os.WriteFile(path, []byte("mobid: 1234\n"), 0o644))
	require.NotNil(t, Load(1234, "testzone"))

	assert.Empty(t, loadFailureLogged["1234:testzone"],
		"a successful load must clear the throttle so a future break logs again")
}

// Control leg: a valid file loads and caches normally.
func TestLoad_ValidFileLoadsAndCaches(t *testing.T) {
	dir := withDialogueDir(t)
	writeDialogue(t, dir, "testzone", 1234, "mobid: 1234\ndefaultMood: friendly\n")

	df := Load(1234, "testzone")
	require.NotNil(t, df)

	_, cached := dialogueCache["1234:testzone"]
	assert.True(t, cached, "a good file must be cached")
}
