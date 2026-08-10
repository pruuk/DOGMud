package questengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Chunk 2.5 / finding 16 — the flag validator must not silently narrow its own
// input.
//
// loadAllDialogueFiles `continue`d past every read and parse error, so
// ValidateAllFlags could report success without having inspected files that may
// contain undeclared or misspelled flag keys — which is the entire thing it
// exists to catch. Undeclared flags are a startup panic by design, so a
// validator that skips files makes that guarantee hollow while still producing
// confidence.
// ---------------------------------------------------------------------------

func writeDialogueFile(t *testing.T, base, zone, name, body string) string {
	t.Helper()
	p := filepath.Join(base, zone, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

func TestLoadAllDialogueFiles_ReportsUnparseableFilesInsteadOfSkipping(t *testing.T) {
	base := t.TempDir()
	writeDialogueFile(t, base, "goodzone", "100.yaml", "mobid: 100\n")
	writeDialogueFile(t, base, "badzone", "200.yaml", "mobid: 200\npatterns: [unclosed\n")

	entries, failures := loadAllDialogueFiles(base)

	assert.Len(t, entries, 1, "the readable file is still collected")
	require.Len(t, failures, 1, "the unparseable file must be reported, not skipped")
	assert.Contains(t, failures[0], "200.yaml")
	assert.Contains(t, failures[0], "cannot parse")
}

// Control leg: a clean tree reports no failures.
func TestLoadAllDialogueFiles_CleanTreeHasNoFailures(t *testing.T) {
	base := t.TempDir()
	writeDialogueFile(t, base, "goodzone", "100.yaml", "mobid: 100\n")
	writeDialogueFile(t, base, "goodzone", "101.yaml", "mobid: 101\n")

	entries, failures := loadAllDialogueFiles(base)

	assert.Len(t, entries, 2)
	assert.Empty(t, failures)
}

// A missing dialogue directory is absence, not corruption, and must not fail
// validation — tests and fresh installs legitimately have none.
func TestLoadAllDialogueFiles_MissingDirectoryIsNotAFailure(t *testing.T) {
	entries, failures := loadAllDialogueFiles(filepath.Join(t.TempDir(), "does-not-exist"))

	assert.Empty(t, entries)
	assert.Empty(t, failures, "no dialogue directory is fine")
}

// Non-YAML files are ignored rather than reported: they are not dialogue.
func TestLoadAllDialogueFiles_NonYamlFilesAreIgnoredNotReported(t *testing.T) {
	base := t.TempDir()
	writeDialogueFile(t, base, "goodzone", "100.yaml", "mobid: 100\n")
	writeDialogueFile(t, base, "goodzone", "notes.txt", "this is not dialogue")

	entries, failures := loadAllDialogueFiles(base)

	assert.Len(t, entries, 1)
	assert.Empty(t, failures)
}
