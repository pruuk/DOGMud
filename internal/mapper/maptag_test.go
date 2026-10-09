package mapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// #455: "Deep Water" put a space inside fg="map-deep water", the tag parser
// gave up, and the raw tag printed as text on every look.
func TestLegendSlug_SpacesBecomeHyphens(t *testing.T) {
	assert.Equal(t, "deep-water", LegendSlug("Deep Water"))
	assert.Equal(t, "city-thoroughfare", LegendSlug("City Thoroughfare"))
	assert.Equal(t, "cave", LegendSlug("Cave"))
}

func TestColorizeLegendLine_DeepWaterTagHasNoSpace(t *testing.T) {
	got := ColorizeLegendLine("║≈─@║", map[rune]string{'≈': "Deep Water", '@': "You"})
	assert.Equal(t,
		`║<ansi fg="map-room"><ansi fg="map-deep-water" bg="mapbg-deep-water">≈</ansi></ansi>─`+
			`<ansi fg="map-room"><ansi fg="map-you" bg="mapbg-you">@</ansi></ansi>║`, got)
}

// One pass, rune by rune: a legend symbol that also occurs inside a tag
// already written ('m' is in "map-room") is never rewritten. The old
// per-symbol strings.Replace loop rewrote it whenever map order put the
// letter's pass after the other symbol's.
func TestColorizeLegendLine_NeverRewritesInsideATag(t *testing.T) {
	legend := map[rune]string{'≈': "Deep Water", 'm': "Mine"}
	for i := 0; i < 20; i++ {
		got := ColorizeLegendLine("≈m", legend)
		assert.Equal(t,
			`<ansi fg="map-room"><ansi fg="map-deep-water" bg="mapbg-deep-water">≈</ansi></ansi>`+
				`<ansi fg="map-room"><ansi fg="map-mine" bg="mapbg-mine">m</ansi></ansi>`, got)
	}
}

// Every shipped multi-word biome has a colour alias under its slug, so the
// hyphenated tag still colours the symbol.
func TestMultiWordBiomesHaveSlugAliases(t *testing.T) {
	for _, world := range []string{"default", "dogmud"} {
		root := filepath.Join("..", "..", "_datafiles", "world", world)
		raw, err := os.ReadFile(filepath.Join(root, "ansi-aliases.yaml"))
		require.NoError(t, err)
		var aliases struct {
			Colors map[string]any `yaml:"colors"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &aliases))
		require.NotEmpty(t, aliases.Colors, world)

		files, err := filepath.Glob(filepath.Join(root, "biomes", "*.yaml"))
		require.NoError(t, err)
		require.NotEmpty(t, files, world)
		for _, f := range files {
			b, err := os.ReadFile(f)
			require.NoError(t, err)
			var biome struct {
				Name string `yaml:"name"`
			}
			require.NoError(t, yaml.Unmarshal(b, &biome))
			if !strings.Contains(biome.Name, " ") {
				continue
			}
			_, ok := aliases.Colors["map-"+LegendSlug(biome.Name)]
			assert.True(t, ok, "%s: biome %q has no map-%s alias", world, biome.Name, LegendSlug(biome.Name))
		}
	}
}
