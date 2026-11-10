package util

import (
	"fmt"
	"strings"
	"testing"
)

// A word wider than the limit gets a line of its own, in its place in the
// sentence.
func TestSplitString_OverLongWordKeepsItsPlace(t *testing.T) {
	long := strings.Repeat("a", 30)
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"mid sentence", "before it " + long + " after it", []string{"before it", long, "after it"}},
		{"first word", long + " after it", []string{long, "after it"}},
		{"last word", "before it " + long, []string{"before it", long}},
		{"two in a row", long + " " + long + " end", []string{long, long, "end"}},
		{"with its punctuation", "before it " + long + ". after it", []string{"before it", long + ".", "after it"}},
		{"a later input line is untouched", long + "\n  indented", []string{long, "  indented"}},
		{
			"its color stays on it, with no empty line",
			`before <ansi fg="red">` + long + `</ansi> after`,
			[]string{`before <ansi fg="red">`, long + `</ansi>`, `after`},
		},
		{
			"an opening tag rides on the word",
			`<ansi fg="red">` + long + `</ansi> after`,
			[]string{`<ansi fg="red">` + long + `</ansi>`, `after`},
		},
		{
			"tags closing the text join the word",
			`<ansi fg="red"><ansi fg="blue">` + long + `</ansi></ansi>`,
			[]string{`<ansi fg="red"><ansi fg="blue">` + long + `</ansi></ansi>`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitString(tc.in, 20)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestSplitString_AnsiClosingTag(t *testing.T) {
	input := fmt.Sprintf("<ansi fg=\"mobname\">Sylara</ansi> says, \"<ansi fg=\"saytext-mob\">Search the bone pile and bring it back. Then I will know you are sincere.'</ansi>\"")

	lines := SplitString(input, 80)
	joined := strings.Join(lines, "\n")

	// The closing tag must not be split across lines
	if strings.Contains(joined, "<\n") || strings.Contains(joined, "\nansi>") {
		t.Errorf("ANSI closing tag was split across lines:\n%s", joined)
	}

	// Verify no raw "ansi>" appears as visible text (tag was destroyed)
	for i, line := range lines {
		vis := ansiTagRegex.ReplaceAllString(line, "")
		if strings.Contains(vis, "ansi>") {
			t.Errorf("Line %d has broken ANSI tag in visible text: %q", i, vis)
		}
	}
}
