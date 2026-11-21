package menu

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// A heading longer than the reader wraps between words, like every other line.
func TestRenderWrapsLongHeading(t *testing.T) {
	const heading = `Kael sets down his whittling. "Ask what you like, I'll answer plain."`
	out := Render(Menu{Heading: heading, Entries: []Entry{{Id: "a", Label: "Hello"}}}, 38)
	words := map[string]bool{}
	for _, w := range strings.Fields(heading) {
		words[w] = true
	}
	headingLines := 0
	for _, line := range strings.Split(out, "\n") {
		p := plain(line)
		if n := utf8.RuneCountInString(p); n > 38 {
			t.Errorf("line over 38 columns (%d): %q", n, p)
		}
		if strings.HasPrefix(p, "  1.") {
			break
		}
		for _, w := range strings.Fields(p) {
			if strings.Contains(heading, w) && !words[w] {
				t.Errorf("word split: %q in line %q", w, p)
			}
		}
		if p != "" {
			headingLines++
		}
	}
	if headingLines < 2 {
		t.Errorf("heading did not wrap: %q", out)
	}
}
