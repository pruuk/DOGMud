package mapper

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

// The map's own ASCII forms apply to map cells only (owner ruling
// 2026-10-17): Deep Water's ≈ reads "D" on an ASCII map while the shared
// table still makes it "~" everywhere else, and Shore's "~" is untouched.
func TestAsciiMapCellsUseTheMapForms(t *testing.T) {
	m := newMapRender(4, 1)
	m.Render[0] = []rune{'≈', '~', '•', '@'}
	legend := map[rune]string{'≈': "Deep Water", '~': "Shore", '•': "Road", '@': "You"}

	m.AsciiCells()
	assert.Equal(t, "D~o@", string(m.Render[0]))
	assert.Equal(t, map[rune]string{'D': "Deep Water", '~': "Shore", 'o': "Road", '@': "You"}, AsciiLegend(legend))

	assert.Equal(t, "~", util.ConvertToAscii("≈"), "the global ASCII table keeps ≈ as ~")
	assert.Equal(t, '-', AsciiMapSymbol('─'), "a glyph with no map form takes the shared form")
}
