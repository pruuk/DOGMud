package mapper

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// #455: "Deep Water" put a space inside fg="map-deep water", which matches
// no colour alias, so the tile lost its colour. (The raw tags the playtest
// saw came from noun highlighting; see rooms.highlightNouns.)
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

// codeLegendPattern matches a legend name the Go code writes onto a map:
// c.OverrideSymbol(room, sym, `Name`) and out.legend[sym] = `Name`.
var codeLegendPattern = regexp.MustCompile("(?:OverrideSymbol\\([^`\\n]*|legend\\[[A-Za-z]+\\] = )`([^`]+)`")

// codeLegendNames reads every legend name the Go code can emit.
func codeLegendNames(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, dir := range []string{"../usercommands", "."} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		require.NotEmpty(t, files, dir)
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			b, err := os.ReadFile(f)
			require.NoError(t, err)
			for _, m := range codeLegendPattern.FindAllStringSubmatch(string(b), -1) {
				names = append(names, m[1])
			}
		}
	}
	// The pattern must be able to see the legends it guards.
	require.Contains(t, names, "Party Member")
	require.Contains(t, names, "Secret")
	return names
}

// roomLegendNames reads every per-room maplegend in one world.
func roomLegendNames(t *testing.T, root string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(root, "rooms", "*", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, files, root)
	var names []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		if !strings.Contains(string(b), "maplegend:") {
			continue
		}
		var room struct {
			MapLegend string `yaml:"maplegend"`
		}
		require.NoError(t, yaml.Unmarshal(b, &room), f)
		if room.MapLegend != "" {
			names = append(names, room.MapLegend)
		}
	}
	return names
}

// Every multi-word legend the game can emit (a room's maplegend, or a name
// the code writes such as "Party Member") has a colour alias under its
// slug, in both worlds. Single-word names slug the same as they lower-case,
// so only a multi-word name can lose its colour to the slug.
func TestEveryMultiWordLegendHasSlugAlias(t *testing.T) {
	code := codeLegendNames(t)
	for _, world := range []string{"default", "dogmud"} {
		root := filepath.Join("..", "..", "_datafiles", "world", world)
		raw, err := os.ReadFile(filepath.Join(root, "ansi-aliases.yaml"))
		require.NoError(t, err)
		var aliases struct {
			Colors map[string]any `yaml:"colors"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &aliases))

		for _, name := range append(roomLegendNames(t, root), code...) {
			if !strings.Contains(name, " ") {
				continue
			}
			_, ok := aliases.Colors["map-"+LegendSlug(name)]
			assert.True(t, ok, "%s: legend %q has no map-%s alias", world, name, LegendSlug(name))
		}
	}
}

// Every template that builds a map colour tag from a legend name uses the
// mapslug function, never lowercase, which leaves a multi-word name's space
// in the tag.
func TestMapTagTemplatesUseMapslug(t *testing.T) {
	for _, world := range []string{"default", "dogmud"} {
		root := filepath.Join("..", "..", "_datafiles", "world", world, "templates")
		count := 0
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".template") {
				return err
			}
			count++
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(b), "\n") {
				if strings.Contains(line, "map-") && strings.Contains(line, "lowercase") {
					t.Errorf("%s:%d builds a map tag with lowercase, want mapslug: %s", path, i+1, strings.TrimSpace(line))
				}
			}
			return nil
		})
		require.NoError(t, err)
		require.NotZero(t, count, root)
	}
}
