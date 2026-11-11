package messaging

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var packTagRE = regexp.MustCompile(`</?ansi[^>]*>`)

// packPlain is lines without their colour tags.
func packPlain(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = packTagRE.ReplaceAllString(l, "")
	}
	return out
}

// A unit pulled left as the last unit leaves no empty line after it.
func TestPackUnitsLeavesNoTrailingEmptyLine(t *testing.T) {
	got := PackUnits([]string{"[WOMG]", "Bartholomewsworthingtonianus"}, 2, 4, 30)
	assert.Equal(t, []string{"  [WOMG]", "  Bartholomewsworthingtonianus"}, got)
}

// A unit with no space that fits the line but not its indent is still pulled
// left to end at the edge in a Go screen (decision 1), a character name too.
func TestPackUnitsPullsLeftASpacelessUnitThatFitsTheLine(t *testing.T) {
	name := "Bartholomewsworthingtonianus"
	for _, names := range [][]string{nil, {name}} {
		got := PackUnits([]string{"led", "by", name}, 4, 4, 30, names...)
		assert.Equal(t, []string{"    led by", "  " + name}, got)
	}
}

// A unit with no space wider than the whole line (a 40-letter guild name, a
// long link in a macro) breaks at the right edge, keeping its colour on
// every piece, as wrapbreak breaks signs and runes. The first piece fills
// the line it starts on; the rest keep the continuation indent.
func TestPackUnitsBreaksASpacelessNonNameWiderThanTheLine(t *testing.T) {
	guild := "Wardens-of-the-Old-Thornwall-Market-Gate"
	got := PackUnits([]string{`<ansi fg="yellow-bold">` + guild + `</ansi>`}, 0, 2, 30)
	assert.Equal(t, []string{guild[:30], "  " + guild[30:]}, packPlain(got))
	for _, line := range got {
		assert.Equal(t, strings.Count(line, "<ansi"), strings.Count(line, "</ansi>"), "unbalanced: %q", line)
		assert.True(t, strings.HasPrefix(strings.TrimLeft(line, " "), `<ansi fg="yellow-bold">`), "lost its colour: %q", line)
	}

	link := "https://example.com/a/very/long/link/that/has/no/spaces/at/all"
	got = PackUnits([]string{"a:", "say", link, "ok"}, 1, 3, 30)
	assert.Equal(t, []string{" a: say", "   " + link[:27], "   " + link[27:54], "   " + link[54:] + " ok"}, got)
}

// A character name wider than the whole line (a legacy 32-letter name, the
// accepted exception of ruling 8) is never broken: it keeps its indent and
// runs past the edge on a line of its own.
func TestPackUnitsLetsALegacyNameOverrunAtItsIndent(t *testing.T) {
	name := "Wilhelmina_Ashgrove_Thornbramble"
	got := PackUnits([]string{"[WOMG]", name, "Leader"}, 2, 4, 30, name)
	assert.Equal(t, []string{"  [WOMG]", "    " + name, "    Leader"}, got)
}

// PackUnitsKeepIndent never pulls a piece left of its indent, and never
// breaks a word: in the two-line table form column 0 starts the next item.
func TestPackUnitsKeepIndentNeverPullsLeft(t *testing.T) {
	name29 := "Bartholomewsworthingtonianusx"
	got := PackUnitsKeepIndent([]string{"Seller: " + name29, "Buyer: Halix"}, 2, 2, 30)
	assert.Equal(t, []string{"  Seller:", "  " + name29, "  Buyer: Halix"}, got)

	name32 := "Wilhelmina_Ashgrove_Thornbramble"
	got = PackUnitsKeepIndent([]string{"Seller: " + name32}, 2, 2, 30)
	assert.Equal(t, []string{"  Seller:", "  " + name32}, got)
	require.NotContains(t, got, "")
}
