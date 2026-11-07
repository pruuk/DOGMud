package conversation

import "testing"

// A `|` block in a dialogue file keeps its authored line breaks, and the
// wrapper then wraps around them: the Keeper's hand-in read "and / the first
// time / it stopped", with its closing quote stranded on a line of its own.
func TestUnbreakLines(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"one sentence over lines", "Yes. I felt it wake, and the first time\nit stopped.\n", "Yes. I felt it wake, and the first time it stopped."},
		{"windows line ends", "one\r\ntwo\r\n", "one two"},
		{"indent and trailing space", "one  \n   two", "one two"},
		{"a blank line stays a paragraph", "one\ntwo\n\nthree\nfour\n", "one two\n\nthree four"},
		{"plain text is untouched", "Into the buried thing, then.", "Into the buried thing, then."},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := unbreakLines(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
