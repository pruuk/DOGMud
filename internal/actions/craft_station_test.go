package actions

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withPortableWorkshop(t *testing.T, owned bool) *characters.Character {
	t.Helper()
	// ⚠️ HasMutationFlag resolves the flag through the LOADED mutation SPECS,
	// not the owned map, so a test binary sees no flag at all unless the
	// registry is seeded. Mirrors walking-chrysalis.yaml, which carries
	// `type: flag, target: portable-workshop` under pros.
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"walking-chrysalis": {
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "portable-workshop"}},
		},
	}))
	c := characters.New()
	if owned {
		c.Mutations = map[string]int{"walking-chrysalis": 1}
	}
	return c
}

func TestStationSatisfied(t *testing.T) {
	t.Run("recipe needs no station", func(t *testing.T) {
		c := withPortableWorkshop(t, false)
		assert.True(t, StationSatisfied(c, "", ""))
		assert.True(t, StationSatisfied(c, "", "forge"))
	})

	t.Run("standing at the right station", func(t *testing.T) {
		c := withPortableWorkshop(t, false)
		assert.True(t, StationSatisfied(c, "forge", "forge"))
	})

	t.Run("wrong station without the mutation is refused", func(t *testing.T) {
		c := withPortableWorkshop(t, false)
		assert.False(t, StationSatisfied(c, "forge", ""))
		assert.False(t, StationSatisfied(c, "forge", "loom"))
	})

	// The reported bug: Walking Chrysalis promises "no forge, no loom, no bench
	// of any kind — make anything, anywhere".
	t.Run("Walking Chrysalis crafts anywhere", func(t *testing.T) {
		c := withPortableWorkshop(t, true)
		require.True(t, mutations.HasPortableWorkshop(c.Mutations), "fixture must actually grant the flag")
		assert.True(t, StationSatisfied(c, "forge", ""))
		assert.True(t, StationSatisfied(c, "forge", "loom"))
		assert.True(t, StationSatisfied(c, "alchemy_bench", ""))
	})

	t.Run("nil character is refused rather than panicking", func(t *testing.T) {
		assert.False(t, StationSatisfied(nil, "forge", ""))
		assert.True(t, StationSatisfied(nil, "", ""), "no station required is still fine")
	})
}

// ⚠️ DRIFT GUARD. The rule was inlined in FIVE places and exactly ONE honoured
// the mutation, which is why the mutation looked completely dead to the player:
// the craft was allowed while the list said `locked`, the status column said
// `need forge`, enchanting refused, and storage would not release components.
//
// Any NEW raw station comparison outside this helper re-creates that split, so
// fail loudly if one appears in the craft command surface.
func TestNoRawStationComparisonsOutsideTheHelper(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")

	files := []string{
		filepath.Join(root, "internal", "usercommands", "craft.go"),
		filepath.Join(root, "internal", "actions", "craft.go"),
	}

	// A gate compares the ROOM's station against the RECIPE's. A display-only
	// `r.Station != ""` (used to render a "[forge]" label) is fine and must not
	// trip this.
	gate := regexp.MustCompile(`room\.Station\s*(!=|==)\s*\w+\.Station`)

	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, "guard must be able to read %s", f)
		body := string(raw)

		// Strip the helper itself, which legitimately contains the comparison.
		body = regexp.MustCompile(`(?s)func StationSatisfied\(.*?\n}`).ReplaceAllString(body, "")
		// Strip comments, which quote the old form on purpose.
		body = regexp.MustCompile(`(?m)^\s*//.*$`).ReplaceAllString(body, "")

		assert.NotRegexp(t, gate, body,
			"%s compares stations directly; route it through actions.StationSatisfied "+
				"or the Walking Chrysalis mutation silently stops working there", filepath.Base(f))
	}
}
