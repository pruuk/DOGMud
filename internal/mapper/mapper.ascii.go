package mapper

import (
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// asciiMapSymbols gives a map cell its own ASCII form where the shared
// ConvertToAscii form would read the same as another legend entry. These
// apply to map cells only: the same rune elsewhere (≈ in the sunset banner's
// waves) keeps the ConvertToAscii form. UTF-8 players never see them.
// TestMapLegendAsciiFormsAreDistinct finds the collisions this table answers.
var asciiMapSymbols = map[rune]rune{
	// Deep Water: ConvertToAscii makes ≈ a "~", which is Shore's own symbol
	// (owner ruling 2026-10-17). W is taken by the Wardstone room (4145).
	'≈': 'D',
	// The town, road, dungeon and land biomes: ConvertToAscii makes • a "*",
	// which is Desert's own symbol (and The Breach's, room 6373).
	'•': 'o',
	// Snow: ConvertToAscii makes ❄ a "*" too.
	'❄': 'n',
	// Cliffs: ConvertToAscii makes ▼ a "v", which is the down-exit arrow
	// (#593). C is taken by a room mapsymbol, so the lowercase initial, as
	// Dense Forest F and Forest f share one letter in two cases.
	'▼': 'c',
}

// AsciiMapSymbol is the one ASCII character an ASCII-mode map draws for r:
// the map's own form when it has one, else ConvertToAscii's when that is one
// ASCII character, else r unchanged.
func AsciiMapSymbol(r rune) rune {
	if a, ok := asciiMapSymbols[r]; ok {
		return a
	}
	if r < utf8.RuneSelf {
		return r
	}
	if s := util.ConvertToAscii(string(r)); len(s) == 1 && s[0] < utf8.RuneSelf {
		return rune(s[0])
	}
	return r
}

// AsciiCells rewrites a rendered map's cells to their AsciiMapSymbol forms,
// for an ASCII-mode player. Call it before the legend is built from the
// cells (AsciiLegend) and before ColorizeLegendLine.
func (m *mapRender) AsciiCells() {
	for y := range m.Render {
		for x, r := range m.Render[y] {
			m.Render[y][x] = AsciiMapSymbol(r)
		}
	}
}

// AsciiLegend re-keys a legend (from GetLegend) by each symbol's
// AsciiMapSymbol form, so it matches cells rewritten by AsciiCells.
func AsciiLegend(legend map[rune]string) map[rune]string {
	out := make(map[rune]string, len(legend))
	for r, name := range legend {
		out[AsciiMapSymbol(r)] = name
	}
	return out
}
