package mapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// #253 follow-up: an ASCII-mode map is converted cell by cell with the rest
// of the line, so a glyph that converts to nothing, or to several
// characters, shifts its row and breaks the frame. Every glyph a map can
// draw must convert to exactly one ASCII character: each biome symbol in
// both worlds, each room mapsymbol in the live dogmud world, the mapper's
// own symbols and its exit lines. (The command's markers are held to it in
// usercommands.) The default world's rooms are upstream sample content and
// use ♜, which has no ASCII form; they are not scanned.
func TestMapGlyphsConvertToOneAsciiCharacter(t *testing.T) {
	glyphs := map[string]string{
		"default symbol": string(defaultMapSymbol),
		"secret":         string(SecretSymbol),
		"locked":         string(LockedSymbol),
	}
	for dir, d := range posDeltas {
		glyphs["exit "+dir] = string(d.arrow)
	}
	for _, world := range []string{"default", "dogmud"} {
		root := filepath.Join("..", "..", "_datafiles", "world", world)

		biomes, err := filepath.Glob(filepath.Join(root, "biomes", "*.yaml"))
		require.NoError(t, err)
		require.NotEmpty(t, biomes, world)
		for _, f := range biomes {
			b, err := os.ReadFile(f)
			require.NoError(t, err)
			var biome struct {
				Symbol string `yaml:"symbol"`
			}
			require.NoError(t, yaml.Unmarshal(b, &biome), f)
			if biome.Symbol != "" {
				glyphs[world+" biome "+filepath.Base(f)] = biome.Symbol
			}
		}

		if world != "dogmud" {
			continue
		}
		roomFiles, err := filepath.Glob(filepath.Join(root, "rooms", "*", "*.yaml"))
		require.NoError(t, err)
		for _, f := range roomFiles {
			b, err := os.ReadFile(f)
			require.NoError(t, err)
			if !strings.Contains(string(b), "mapsymbol:") {
				continue
			}
			var room struct {
				MapSymbol string `yaml:"mapsymbol"`
			}
			require.NoError(t, yaml.Unmarshal(b, &room), f)
			if room.MapSymbol != "" {
				glyphs[world+" room "+filepath.Base(f)] = room.MapSymbol
			}
		}
	}
	// The scan must reach the glyphs it guards.
	require.Contains(t, glyphs, "dogmud biome cave.yaml")
	require.Contains(t, glyphs, "dogmud room 480.yaml")

	for name, g := range glyphs {
		got := util.ConvertToAscii(g)
		assert.True(t, len(got) == 1 && got[0] < 0x80,
			"%s: %q converts to %q, not one ASCII character", name, g, got)
	}
}
