package species

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// dogmudSpecies reads the DOGMUD species files directly.
//
// LoadForTest cannot be used here. It resolves configs.GetFilePathsConfig()
// .DataFiles, which defaults to `_datafiles/world/default` in a test binary, so
// it both panics from a package working directory AND points at UPSTREAM's
// roster rather than dogmud's. A test that used it would be validating the
// wrong world.
func dogmudSpecies(t *testing.T) map[int]*Species {
	t.Helper()
	dir := filepath.Join("..", "..", "_datafiles", "world", "dogmud", "species")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	out := map[int]*Species{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		sp := &Species{}
		require.NoError(t, yaml.Unmarshal(data, sp), "parse %s", e.Name())
		out[sp.SpeciesId] = sp
	}
	require.NotEmpty(t, out, "no species parsed: the walk is broken, not the data")
	return out
}

// Eight species declare buff 29, and the buff was absent from dogmud entirely,
// so 67 mobs were silently blind in their own caves. This pins the list so a
// later data edit cannot quietly shrink it.
func TestNightVisionSpeciesDeclareBuff29(t *testing.T) {
	all := dogmudSpecies(t)
	want := []int{2, 4, 5, 8, 9, 11, 17, 24}
	for _, id := range want {
		sp := all[id]
		require.NotNil(t, sp, "species %d missing", id)
		require.Contains(t, sp.BuffIds, 29, "species %d (%s) should declare night vision", id, sp.Name)
	}
}

// The regression itself: every buff a dogmud species references must exist as a
// dogmud buff file. Buff 29 failed this for months and nothing noticed.
func TestEverySpeciesBuffIdHasADogmudFile(t *testing.T) {
	all := dogmudSpecies(t)
	buffDir := filepath.Join("..", "..", "_datafiles", "world", "dogmud", "buffs")
	for id, sp := range all {
		for _, bid := range sp.BuffIds {
			matches, err := filepath.Glob(filepath.Join(buffDir, strconv.Itoa(bid)+"-*.yaml"))
			require.NoError(t, err)
			require.NotEmpty(t, matches,
				"species %d (%s) references buff %d, which has no file in dogmud/buffs", id, sp.Name, bid)
		}
	}
}
