package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse_SectionsAndErrorsNameTheFile(t *testing.T) {
	m, err := Parse("step-ids.txt", []byte("# header\n53-start\n53-end\n# retired\n53-old\n# renamed\n53-a -> 53-b\n"))
	require.NoError(t, err)
	require.Equal(t, []string{"53-end", "53-start"}, m.Keys)
	require.Equal(t, []string{"53-old"}, m.Retired)
	require.Equal(t, map[string]string{"53-a": "53-b"}, m.Renamed)

	_, err = Parse("step-ids.txt", []byte("53-start\n53-start\n"))
	require.ErrorContains(t, err, "step-ids.txt line 2")
}

func TestDiff_MissingAndUnlisted(t *testing.T) {
	m := Manifest{Keys: []string{"a", "b", "gone"}, Retired: []string{"old"}, Renamed: map[string]string{"r": "b"}}
	missing, unlisted := m.Diff([]string{"a", "b", "new"})
	require.Equal(t, []string{"gone"}, missing)
	require.Equal(t, []string{"new"}, unlisted)
}
