package templates

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// plainLines is the visible text of each line of s, colour tags removed.
func plainLines(s string) []string {
	return strings.Split(ansiTag.ReplaceAllString(s, ""), "\n")
}

// A name (a top-level colour span) is never split while it fits a line by
// itself, even when the text glued to it makes the unit wider than a line
// (owner, 2026-11-10).
func TestWrapSpansKeepsFittingNameWhole(t *testing.T) {
	const riss = `<ansi fg="mobname">Riss the Cartographer-Scholar</ansi>`
	const odell = `<ansi fg="mobname">Mistress Odell the Victualler</ansi>`
	for _, tc := range []struct {
		name, in string
		width    int
		want     []string
	}{
		{"glue after", `Tracking ` + riss + `... They are here!`, 30,
			[]string{"Tracking", "Riss the Cartographer-Scholar", "... They are here!"}},
		{"glue before", `Here: (` + odell + `) is`, 30,
			[]string{"Here:", "(Mistress Odell the Victualler", ") is"}},
		{"glue both sides, both forced", `Here: (` + odell + `) is`, 29,
			[]string{"Here: (", "Mistress Odell the Victualler", ") is"}},
		{"glue before only", `Here: (` + odell + ` is`, 29,
			[]string{"Here: (", "Mistress Odell the Victualler", "is"}},
		{"fits whole, untouched", `Tracking ` + riss + `... ok`, 50,
			[]string{"Tracking Riss the Cartographer-Scholar... ok"}},
		{"span wider than a line still wraps", `See ` + riss + `... now`, 20,
			[]string{"See Riss the", "Cartographer-Scholar...", "now"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapSpans(tc.in, tc.width)
			assert.Equal(t, tc.want, plainLines(got), got)
			for _, line := range strings.Split(got, "\n") {
				assert.Equal(t, strings.Count(line, "<ansi"), strings.Count(line, "</ansi>"), "unbalanced: %q", line)
			}
		})
	}
}

// A name inside another colour span (an item in a coloured sentence, a count
// in a coloured notice) is a unit too, kept whole while it fits a line, with
// what is glued to it (plan 3b-3).
func TestWrapSpansKeepsNestedNameWhole(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		width    int
		want     []string
	}{
		{"name in a sentence", `<ansi fg="mail-note">This message came with one <ansi fg="itemname">Hull-Plate Cuirass</ansi> attached.</ansi>`, 38,
			[]string{"This message came with one", "Hull-Plate Cuirass attached."}},
		{"count with its glue", `<ansi fg="yellow">The auction will end in <ansi fg="white-bold">171 seconds</ansi>!</ansi>`, 28,
			[]string{"The auction will end in", "171 seconds!"}},
		{"outer span that fits stays whole", `<ansi fg="yellow">ends in <ansi fg="white-bold">171 seconds</ansi>!</ansi> now`, 21,
			[]string{"ends in 171 seconds!", "now"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapSpans(tc.in, tc.width)
			assert.Equal(t, tc.want, plainLines(got), got)
			for _, line := range strings.Split(got, "\n") {
				assert.Equal(t, strings.Count(line, "<ansi"), strings.Count(line, "</ansi>"), "unbalanced: %q", line)
			}
		})
	}
}

// A unit that fits is byte-identical to wrapText's output.
func TestWrapSpansFittingUnitUnchanged(t *testing.T) {
	in := `Tracking <ansi fg="mobname">Riss the Cartographer-Scholar</ansi>... They are here and it is a long road ahead.`
	assert.Equal(t, strings.ReplaceAll(wrapText(in, 45), "\x1f", " "), wrapSpans(in, 45))
}

// wrapbreak counts columns from the start of a line, a newline included.
func TestWrapBreakResetsOnNewline(t *testing.T) {
	assert.Equal(t, "abc\ndefgh\nij", wrapBreak("abc\ndefgh ij", 5))
}
