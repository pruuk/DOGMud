package util

import (
	"strings"
	"testing"
)

// A closing quote or closing punctuation stays on the line of the word it
// follows, even with the </ansi> that ends a colored span between them (#603).
// The say text of a mob whisper is built `"<ansi ...>TEXT</ansi>"`, so the
// quote is a token of its own, after the close tag.
func TestSplitString_ClosingPunctuationStaysWithLastWord(t *testing.T) {
	const open = `<ansi fg="saytext-mob">`
	const cls = `</ansi>`

	for _, width := range []int{80, 42, 30} {
		for _, tail := range []string{`.` + cls + `"`, `!` + cls + `"`, `?` + cls + `"`, cls + `."`, `.` + cls + `")`} {
			// Sweep the last word across the width so one length lands exactly
			// on the last column, whatever the tail.
			for n := width - 8; n <= width+2; n++ {
				for _, lead := range []string{``, `aa `, `Go find the `} {
					msg := open + lead + strings.Repeat("a", n) + tail
					for _, line := range SplitString(msg, width) {
						vis := ansiTagRegex.ReplaceAllString(line, ``)
						if strings.TrimSpace(vis) != "" && strings.Trim(vis, `."!?) `) == "" {
							t.Fatalf("width %d tail %q n %d lead %q: a line holds only closing punctuation %q in %q",
								width, tail, n, lead, vis, SplitString(msg, width))
						}
					}
				}
			}
		}
	}
}

// The same rule on the exact shape the issue reports: the last word and its
// period fill the line, then the close tag and the quote.
func TestSplitString_ClosingQuoteAfterCloseTagAtFullLine(t *testing.T) {
	// Quote fits: one line, unchanged.
	word := strings.Repeat("a", 78)
	msg := `<ansi fg="saytext-mob">` + word + `.</ansi>"`
	got := SplitString(msg, 80)
	if len(got) != 1 || got[0] != msg {
		t.Fatalf("got %q, want %q", got, msg)
	}

	// "bb " + 76 a + "." is 80 columns; the quote would be the 81st. The last
	// word moves down with its quote rather than leaving the quote alone.
	msg = `<ansi fg="saytext-mob">bb ` + strings.Repeat("a", 76) + `.</ansi>"`
	got = SplitString(msg, 80)
	want := []string{`<ansi fg="saytext-mob">bb`, strings.Repeat("a", 76) + `.</ansi>"`}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %q, want %q", got, want)
	}
}
