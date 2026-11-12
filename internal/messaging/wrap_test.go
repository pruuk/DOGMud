package messaging

import (
	"strings"
	"testing"
)

func TestWrapAnsiShortLineUnchanged(t *testing.T) {
	got := WrapAnsi("short", 80)
	if got != "short" {
		t.Fatalf("short line should be unchanged, got %q", got)
	}
}

func TestWrapAnsiWrapsAtMaxWidthWordBoundary(t *testing.T) {
	// 20-col wrap, ~40 chars of content
	got := WrapAnsi("the quick brown fox jumps over the lazy dog", 20)
	// Expect at least one newline, and no line longer than 20 visible chars.
	lines := splitLines(got)
	if len(lines) < 2 {
		t.Fatalf("expected wrap to produce >=2 lines, got: %q", got)
	}
	for i, line := range lines {
		if displayWidth(line) > 20 {
			t.Fatalf("line %d exceeds 20 cols (%d): %q", i, displayWidth(line), line)
		}
	}
}

func TestWrapAnsiIgnoresAnsiTagsInWidth(t *testing.T) {
	// 12 visible chars wrapped inside a long ANSI tag.
	input := `<ansi fg="hit-melee">strikes hard</ansi>`
	got := WrapAnsi(input, 80)
	if got != input {
		t.Fatalf("12-visible-char line wrapped at 80 must be unchanged, got %q", got)
	}
}

func TestWrapAnsiCarriesOpenTagAcrossLineBreak(t *testing.T) {
	// 10-col wrap forces a break inside an open tag.
	input := `<ansi fg="hit-melee">strikes deeply at the heart</ansi>`
	got := WrapAnsi(input, 10)
	// First line should END with </ansi>; second line should START
	// with <ansi fg="hit-melee">.
	lines := splitLines(got)
	if len(lines) < 2 {
		t.Fatalf("expected break, got 1 line: %q", got)
	}
	if !endsWith(lines[0], `</ansi>`) {
		t.Fatalf("first line must close the open tag, got %q", lines[0])
	}
	if !startsWith(lines[1], `<ansi fg="hit-melee">`) {
		t.Fatalf("second line must reopen the tag, got %q", lines[1])
	}
}

func TestWrapAnsiMalformedTagFallback(t *testing.T) {
	// Orphan opening tag — wrapper must not panic; should fall back
	// to byte-count wrap.
	input := `<ansi fg="bad" missing close ` +
		`this is fifty-plus characters of unwrapped text after`
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("WrapAnsi panicked on malformed input: %v", r)
		}
	}()
	_ = WrapAnsi(input, 20)
}

func TestWrapAnsiZeroWidthIsPassthrough(t *testing.T) {
	// LineWidth=0 (unset) must not wrap or hang.
	got := WrapAnsi("a long line that would otherwise wrap", 0)
	if got != "a long line that would otherwise wrap" {
		t.Fatalf("LineWidth=0 must pass through, got %q", got)
	}
}

func TestWrapAnsiExplicitNewlineInsideOpenTagNoLeadingSpace(t *testing.T) {
	got := WrapAnsi(`<ansi fg="red">line one`+"\n"+`line two</ansi>`, 80)
	lines := splitLines(got)
	if len(lines) < 2 {
		t.Fatalf("expected 2 lines after explicit newline, got %d: %q", len(lines), got)
	}
	// Second line must start with the reopener tag followed directly
	// by a visible character — no leading space.
	if !startsWith(lines[1], `<ansi fg="red">line`) {
		t.Fatalf("explicit-newline second line had leading space (or wrong reopen): %q", lines[1])
	}
}

func TestWrapAnsiRegexCoversBgAndCombinedAttrs(t *testing.T) {
	// <ansi bg="…"> alone.
	bg := `<ansi bg="black">moonlight</ansi>`
	got := WrapAnsi(bg, 80)
	if got != bg {
		t.Fatalf("bg-only tag should pass through 80-col wrap, got %q", got)
	}
	// Combined fg + bg.
	combo := `<ansi fg="red" bg="black">alert</ansi>`
	got = WrapAnsi(combo, 80)
	if got != combo {
		t.Fatalf("fg+bg combo should pass through 80-col wrap, got %q", got)
	}
	// Width-counting must exclude the tag content. 9 visible chars
	// ("moonlight") with maxWidth=20 stays single-line.
	if c := displayWidth(got); c > 20 {
		t.Fatalf("displayWidth %d unexpectedly high — regex may not be stripping the combined tag", c)
	}
}

// Test helpers — kept here, not exported.

func splitLines(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, cur)
	return out
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// TestDisplayWidthCountsRunesNotBytes pins the test oracle itself. Every
// width assertion in this package leans on displayWidth, so an oracle that
// counts bytes makes those assertions meaningless for any non-ASCII text.
// Fixed before WrapAnsi's own byte-counting bug, so that fix has a truthful
// judge. Matches internal/messaging/hidenames.go:70, which already decodes
// runes correctly in this same package.
func TestDisplayWidthCountsRunesNotBytes(t *testing.T) {
	// Twenty two-byte runes: 40 bytes, 20 visible columns.
	s := strings.Repeat("é", 20) // e-acute, 2 bytes each in UTF-8
	if got := displayWidth(s); got != 20 {
		t.Fatalf("displayWidth(20 two-byte runes) = %d, want 20: the oracle is counting bytes", got)
	}

	// Same, wrapped in a tag the oracle must not count.
	tagged := `<ansi fg="username">` + s + `</ansi>`
	if got := displayWidth(tagged); got != 20 {
		t.Fatalf("displayWidth(tagged) = %d, want 20: tag content must not count toward width", got)
	}
}

func displayWidth(s string) int {
	// Inline scan to count visible RUNES (skip <ansi …> and </ansi>).
	// Ranging over a string decodes runes; indexing it would count bytes
	// and make every width assertion in this file lie about non-ASCII
	// text. See hidenames.go, which decodes runes for the same reason.
	w := 0
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			w++
		}
	}
	return w
}

// A login prompt wrapped for a phone keeps its ": " on its last line, so the
// cursor sits after it, keeps every word, and keeps its colour tags
// balanced. A prompt that fits is unchanged byte for byte.
func TestWrapPromptKeepsTheCursorAfterTheColon(t *testing.T) {
	prompt := "<ansi fg=\"8\">AI players: connect on port 55555. Using this port as an AI risks a ban.</ansi>\r\n" +
		`<ansi fg="39">Username</ansi><ansi fg="black-bold"> (or "</ansi><ansi fg="green-bold">new</ansi><ansi fg="black-bold">"): </ansi>`
	yn := `<ansi fg="yellow-bold">Would you like to create a new user/login named <ansi fg="magenta">Bartholomew_Thistlewood_Iron</ansi>?</ansi> ` +
		`<ansi fg="black-bold">[<ansi fg="green-bold">y</ansi>/<ansi fg="red-bold">n</ansi>]: </ansi>`
	words := func(s string) string { return strings.Join(strings.Fields(stripTags(s)), " ") }
	if got := WrapPrompt(prompt, 80); got != prompt {
		t.Errorf("a prompt that fits 80 changed:\n%q\n%q", prompt, got)
	}
	for _, text := range []string{prompt, yn} {
		for width := 30; width <= 80; width++ {
			got := WrapPrompt(text, width)
			lines := strings.Split(got, "\n")
			for i, line := range lines {
				if w := displayWidth(strings.TrimSuffix(line, "\r")); w > width {
					t.Errorf("at %d line %d is %d wide: %q", width, i+1, w, line)
				}
			}
			last := stripTags(lines[len(lines)-1])
			if !strings.HasSuffix(last, ": ") || strings.TrimSpace(last) == ":" {
				t.Errorf("at %d the last line %q does not end in its prompt and \": \"", width, last)
			}
			if words(got) != words(text) {
				t.Errorf("at %d the words changed:\n%s\n%s", width, words(text), words(got))
			}
			if o, c := strings.Count(got, "<ansi"), strings.Count(got, "</ansi>"); o != c {
				t.Errorf("at %d the wrap opens %d tags and closes %d: %q", width, o, c, got)
			}
		}
	}
}

// When the prompt's last word ends exactly at the width, its ": " has no
// room, so the text is wrapped narrower and the word moves down with it.
func TestWrapPromptMakesRoomForTheTrailingSpace(t *testing.T) {
	text := `<ansi fg="39">aaaa bbbb cc:</ansi><ansi fg="black-bold"> </ansi>`
	// "aaaa bbbb cc:" is 13 wide: at 13 a plain wrap leaves no room for the space.
	got := WrapPrompt(text, 13)
	lines := strings.Split(got, "\n")
	last := stripTags(lines[len(lines)-1])
	if last != "cc: " {
		t.Fatalf("last line = %q, want %q (whole: %q)", last, "cc: ", got)
	}
	for i, line := range lines {
		if w := displayWidth(line); w > 13 {
			t.Errorf("line %d is %d wide: %q", i+1, w, line)
		}
	}
}

// stripTags removes <ansi …> and </ansi> tags.
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>' && inTag:
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestWrapAnsiReopensNestedTagsAcrossBreak is the regression test for the
// scalar-openTag bug. The input is the exact shape applyCategoryColor
// produces: an outer category tag around prose that already carries an
// inner item tag. Before the stack fix, the inner </ansi> cleared the
// tracker and every continuation line rendered uncolored.
func TestWrapAnsiReopensNestedTagsAcrossBreak(t *testing.T) {
	in := `<ansi fg="214">The <ansi fg="item">iron sword</ansi> shatters into ` +
		`a thousand bright splinters that scatter across the floor</ansi>`

	got := WrapAnsi(in, 40)
	lines := splitLines(got)
	if len(lines) < 2 {
		t.Fatalf("expected the input to wrap into several lines, got %d:\n%s", len(lines), got)
	}

	// Every line after the first is still inside the outer fg="214" span,
	// so each must open with a reopener.
	for i, line := range lines[1:] {
		if !startsWith(line, `<ansi fg="214">`) {
			t.Errorf("line %d does not reopen the outer span: %q", i+2, line)
		}
	}

	// Every line must also be balanced: as many opens as closes.
	for i, line := range lines {
		opens := strings.Count(line, "<ansi")
		closes := strings.Count(line, "</ansi>")
		if opens != closes {
			t.Errorf("line %d is unbalanced (%d opens, %d closes): %q", i+1, opens, closes, line)
		}
	}
}

// TestWrapAnsiHandlesThreeLevelNesting proves the stack is a stack and not
// a two-slot special case.
func TestWrapAnsiHandlesThreeLevelNesting(t *testing.T) {
	in := `<ansi fg="214">alpha <ansi fg="item">bravo <ansi fg="username">charlie</ansi> ` +
		`delta</ansi> echo foxtrot golf hotel india juliet kilo lima</ansi>`

	got := WrapAnsi(in, 30)
	for i, line := range splitLines(got) {
		opens := strings.Count(line, "<ansi")
		closes := strings.Count(line, "</ansi>")
		if opens != closes {
			t.Errorf("line %d is unbalanced (%d opens, %d closes): %q", i+1, opens, closes, line)
		}
	}
}

// TestWrapAnsiCountsRunesNotBytes pins the width unit. Reproduced against
// the old code: twenty two-byte runes plus "t ail" broke at visible column
// 21 against a requested width of 40, because each two-byte rune counted
// as two columns.
func TestWrapAnsiCountsRunesNotBytes(t *testing.T) {
	word := strings.Repeat("\u00e9", 20) // e-acute, 2 bytes each
	in := word + "t ail"                 // 21 visible columns, then a space, then 3 more

	got := WrapAnsi(in, 40)
	if strings.Contains(got, "\n") {
		t.Fatalf("25 visible columns must not wrap at width 40, got:\n%q", got)
	}

	// And a multi-byte character sitting exactly at the boundary.
	boundary := word + " " + word // 20 + 1 + 20 = 41 visible columns
	wrapped := WrapAnsi(boundary, 40)
	lines := splitLines(wrapped)
	if len(lines) != 2 {
		t.Fatalf("41 visible columns at width 40 must wrap into 2 lines, got %d:\n%q", len(lines), wrapped)
	}
	for i, line := range lines {
		if w := displayWidth(line); w > 40 {
			t.Errorf("line %d is %d visible columns, over the 40 requested: %q", i+1, w, line)
		}
	}
}
