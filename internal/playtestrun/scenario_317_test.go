package playtestrun

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The #317 ambient scenario parses: four seats, a 50 minute wall clock,
// every seat's goals file and ephemeral profile valid.
func TestParseScenario_317Ambient(t *testing.T) {
	checkout, playtestRoot := repoPlaytestRoot(t)
	got, err := ParseScenario(filepath.Join(playtestRoot, "scenarios", "317-ambient.yaml"), playtestRoot,
		ScenarioParseOpts{Checkout: checkout})
	require.NoError(t, err)
	require.Len(t, got.Roster, 4)
	require.Equal(t, 50*time.Minute, got.WallClock)
}
