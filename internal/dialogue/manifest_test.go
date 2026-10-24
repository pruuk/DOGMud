package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const manifestFixture = `# Topic-key manifest (#375).
94.b
94.a
12.gone
12.old
# retired
12.gone
# renamed
12.old -> 12.new
`

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest([]byte(manifestFixture))
	require.NoError(t, err)
	require.Equal(t, []string{"12.gone", "12.old", "94.a", "94.b"}, m.Keys)
	require.Equal(t, []string{"12.gone"}, m.Retired)
	require.Equal(t, map[string]string{"12.old": "12.new"}, m.Renamed)
}

func TestParseManifest_BOMAndCRLF(t *testing.T) {
	bom := "\xEF\xBB\xBF"
	m, err := ParseManifest([]byte(bom + "94.a\r\n# renamed\r\n12.old -> 12.new\r\n"))
	require.NoError(t, err)
	require.Equal(t, []string{"94.a"}, m.Keys)
	require.Equal(t, map[string]string{"12.old": "12.new"}, m.Renamed)
}

func TestParseManifest_Refusals(t *testing.T) {
	for name, text := range map[string]string{
		"malformed rename":     "# renamed\n12.old 12.new\n",
		"empty right side":     "# renamed\n12.old ->\n",
		"empty left side":      "# renamed\n-> 12.new\n",
		"repeated key":         "94.a\n94.a\n",
		"repeated rename src":  "# renamed\n1.a -> 1.b\n1.a -> 1.c\n",
		"whitespace in key":    "94.a b\n",
		"arrow in key":         "94.a->b\n",
		"whitespace in rename": "# renamed\n1.a -> 1.b c\n",
		"arrow in rename":      "# renamed\n1.a -> 1.b -> 1.c\n",
	} {
		_, err := ParseManifest([]byte(text))
		require.Error(t, err, name)
	}
}

func TestManifestDiff(t *testing.T) {
	m, err := ParseManifest([]byte(manifestFixture))
	require.NoError(t, err)
	missing, unlisted := m.Diff([]string{"94.a", "94.c"})
	require.Equal(t, []string{"94.b"}, missing, "shipped key gone with no retired/renamed line")
	require.Equal(t, []string{"94.c"}, unlisted)
}
