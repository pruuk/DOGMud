package usercommands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// legendGlyph is one symbol a map can draw with the legend name it shows.
type legendGlyph struct {
	symbol rune
	name   string
	source string
	ascii  rune // 0: mapper.AsciiMapSymbol(symbol)
}

// shippedLegendGlyphs lists every legend entry a live-world map can draw: each
// biome symbol, each room mapsymbol, the mapper's own symbols and the map
// command's markers.
func shippedLegendGlyphs(t *testing.T) []legendGlyph {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	world := filepath.Join(filepath.Dir(file), "..", "..", "_datafiles", "world", "dogmud")

	var out []legendGlyph
	add := func(sym string, name, source string) {
		if r := []rune(sym); len(r) > 0 {
			out = append(out, legendGlyph{r[0], name, source, 0})
		}
	}

	biomes, err := filepath.Glob(filepath.Join(world, "biomes", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, biomes)
	for _, f := range biomes {
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		var biome struct {
			Name   string `yaml:"name"`
			Symbol string `yaml:"symbol"`
		}
		require.NoError(t, yaml.Unmarshal(b, &biome), f)
		add(biome.Symbol, biome.Name, "biome "+filepath.Base(f))
	}

	roomFiles, err := filepath.Glob(filepath.Join(world, "rooms", "*", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, roomFiles)
	for _, f := range roomFiles {
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		if !strings.Contains(string(b), "mapsymbol:") {
			continue
		}
		var room struct {
			MapSymbol string `yaml:"mapsymbol"`
			MapLegend string `yaml:"maplegend"`
		}
		require.NoError(t, yaml.Unmarshal(b, &room), f)
		add(room.MapSymbol, room.MapLegend, "room "+filepath.Base(f))
	}

	// keywords.yaml legend-overrides are not glyphs of their own: they only
	// rename the legend of a symbol something above already draws.

	add(string(mapper.SecretSymbol), "Secret", "mapper")
	add(string(mapper.LockedSymbol), "Locked", "mapper")
	add("@", "You", "map command")
	add("☺", "Player", "map command")
	add("☹", "Friend", "map command")
	// The mob marker picks its ASCII form at the source (mobMapSymbol).
	out = append(out, legendGlyph{mobMapSymbol(false), "Mob", "map command", mobMapSymbol(true)})
	return out
}

// TestMapLegendAsciiFormsAreDistinct: on an ASCII-mode map, two legend entries
// drawn with different glyphs must not share one ASCII character, or the
// player cannot tell them apart. The 2026-10-17 playtest saw Shore and Deep
// Water both drawn "~". Entries that already share one glyph in UTF-8 (the
// ☺ markers) are not an ASCII collision. Fix a collision in mapper's
// asciiMapSymbols (map cells only), never in the global ASCII table.
func TestMapLegendAsciiFormsAreDistinct(t *testing.T) {
	glyphs := shippedLegendGlyphs(t)
	byAscii := map[rune]map[rune][]string{}
	for _, g := range glyphs {
		a := g.ascii
		if a == 0 {
			a = mapper.AsciiMapSymbol(g.symbol)
		}
		if a >= 0x80 {
			t.Errorf("%s %q (%s) has no one-character ASCII form", g.source, g.symbol, g.name)
			continue
		}
		if byAscii[a] == nil {
			byAscii[a] = map[rune][]string{}
		}
		byAscii[a][g.symbol] = append(byAscii[a][g.symbol], g.name+" ("+g.source+")")
	}
	var bad []string
	for a, bySym := range byAscii {
		if len(bySym) < 2 {
			continue
		}
		var parts []string
		for sym, names := range bySym {
			sort.Strings(names)
			parts = append(parts, fmt.Sprintf("%q: %s", sym, strings.Join(uniq(names), ", ")))
		}
		sort.Strings(parts)
		bad = append(bad, fmt.Sprintf("%q <- %s", a, strings.Join(parts, " | ")))
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d ASCII map glyphs are drawn for different legend entries:\n%s", len(bad), strings.Join(bad, "\n"))
	}
}

func uniq(in []string) []string {
	var out []string
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
		if len(out) == 3 {
			out = append(out, "...")
			break
		}
	}
	return out
}
