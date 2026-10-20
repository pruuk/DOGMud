package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const manifestFixture = `# Topic-key manifest (#375).
94.a
94.b
# retired
12.gone
# renamed
12.old -> 12.new
`

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest([]byte(manifestFixture))
	require.NoError(t, err)
	require.Equal(t, []string{"94.a", "94.b"}, m.Keys)
	require.Equal(t, []string{"12.gone"}, m.Retired)
	require.Equal(t, map[string]string{"12.old": "12.new"}, m.Renamed)
}

func TestParseManifest_RefusesMalformedRename(t *testing.T) {
	_, err := ParseManifest([]byte("# renamed\n12.old 12.new\n"))
	require.Error(t, err)
}

func TestManifestDiff(t *testing.T) {
	m, err := ParseManifest([]byte(manifestFixture))
	require.NoError(t, err)
	missing, unlisted := m.Diff([]string{"94.a", "94.c"})
	require.Equal(t, []string{"94.b"}, missing, "shipped key gone with no retired/renamed line")
	require.Equal(t, []string{"94.c"}, unlisted)
}
