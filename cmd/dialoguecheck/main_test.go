package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRun_ValidationProblemsFailTheRun runs the whole command against the
// shipped quest and item registries and a fixture dialogue folder whose one
// topic names a quest that does not exist. It loads the real data, so it
// takes a few seconds.
func TestRun_ValidationProblemsFailTheRun(t *testing.T) {
	data := filepath.Join(testdataDir(t, ""), "..", "..", "..", "_datafiles", "world", "dogmud")
	var out bytes.Buffer
	code, err := run(&out, options{
		Data:     data,
		Dialogue: filepath.Join(testdataDir(t, "validation"), "dialogue"),
		Report:   "none",
	})
	require.NoError(t, err)
	require.Equal(t, 1, code, out.String())
	require.Contains(t, out.String(), `VALIDATION: mob 100 topic "ghost": quest 99999 does not exist`)
	require.Contains(t, out.String(), "# no converter report applied")
}
