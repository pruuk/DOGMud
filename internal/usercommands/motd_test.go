package usercommands

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestMotdBoxIsExactlyTheLineWidth is the 2026-10-17 playtest finding "MOTD
// box borders are 81 chars": the border rows ran one column wider than the
// content rows, past an 80-column client.
func TestMotdBoxIsExactlyTheLineWidth(t *testing.T) {
	// The curly quotes and the accent are #592: padding by bytes left any
	// line holding a non-ASCII character short of the right border.
	text := "Welcome to Delusions of Grandeur. Read the rules, mind the " +
		"newbie zone hazards, and ask a mod if anything looks broken.\n" +
		"“The café is open,” says the crier."
	// The box is drawn at 80 and wider (plan 3b-3); a wider telnet screen
	// gets it at its own width.
	for _, lw := range []int{80, 100} {
		box := motdBox(text, lw)
		lines := strings.Split(strings.TrimRight(box, "\n"), "\n")
		if len(lines) < 5 {
			t.Fatalf("lw %d: box has %d lines, fixture is degenerate:\n%s", lw, len(lines), box)
		}
		for i, line := range lines {
			visible := motdAnsiTagRE.ReplaceAllString(line, "")
			if w := utf8.RuneCountInString(visible); w != lw {
				t.Errorf("lw %d line %d is %d columns, want %d: %q", lw, i, w, lw, visible)
			}
		}
	}
}

// TestMotdForScreenReaderIsPlainWords: a screen reader account gets the text
// under a spoken heading, with no box characters and no letter-spaced title;
// everyone else gets motdBox unchanged (#733).
func TestMotdForScreenReaderIsPlainWords(t *testing.T) {
	text := "Welcome to Delusions of Grandeur. Read the rules, mind the " +
		"newbie zone hazards, and ask a mod if anything looks broken."
	for _, lw := range []int{80, 42} {
		out := motdFor(text, lw, true)
		if !strings.Contains(out, "Message of the day") {
			t.Errorf("lw %d: missing the heading:\n%s", lw, out)
		}
		if !strings.Contains(out, "Welcome to Delusions of Grandeur.") {
			t.Errorf("lw %d: missing the MOTD text:\n%s", lw, out)
		}
		for _, bad := range []string{"║", "╠", "╣", "═", "╔", "╗", "╚", "╝", "M E S S A G E"} {
			if strings.Contains(out, bad) {
				t.Errorf("lw %d: screen reader MOTD holds %q:\n%s", lw, bad, out)
			}
		}
		if got, want := motdFor(text, lw, false), motdBox(text, lw); got != want {
			t.Errorf("lw %d: non-screen-reader MOTD is not the box:\n%s", lw, got)
		}
	}
}

// The narrow form: below 80 columns, or with a word wider than the box, the
// MOTD is the ruled form (rules, the title, the text), with no box to burst
// and no corners (plan 3b-3, ruling 1).
func TestMotdNarrowFormFitsTheScreen(t *testing.T) {
	text := "Welcome to Delusions of Grandeur. Read the rules, mind the " +
		"newbie zone hazards, and ask a mod if anything looks broken."
	for _, lw := range []int{30, 42, 44, 45, 79} {
		out := motdBox(text, lw)
		if strings.Contains(out, "╔") {
			t.Errorf("lw %d: still boxed:\n%s", lw, out)
		}
		for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if w := motdVisibleWidth(motdAnsiTagRE.ReplaceAllString(line, "")); w > lw {
				t.Errorf("lw %d line %d is %d columns: %q", lw, i, w, line)
			}
		}
	}
	link := "Join us: https://example.org/a/very/long/path/that/does/not/fit/inside/the/box/at/all/x"
	out := motdBox(link, 80)
	if strings.Contains(out, "╔") {
		t.Errorf("a word wider than the box gets the ruled form:\n%s", out)
	}
	if !strings.Contains(out, strings.Repeat("═", 80)) {
		t.Errorf("at 80 the ruled form's rules are 80 wide:\n%s", out)
	}
}
