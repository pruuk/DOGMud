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
	for _, lw := range []int{80, 55} {
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

// The narrow form: below the width the title fits, or with a word wider than
// the box, the MOTD is a plain title and wrapped text, with no box to burst.
func TestMotdNarrowFormFitsTheScreen(t *testing.T) {
	text := "Welcome to Delusions of Grandeur. Read the rules, mind the " +
		"newbie zone hazards, and ask a mod if anything looks broken."
	for _, lw := range []int{30, 42, 44} {
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
	if out := motdBox(text, 45); !strings.Contains(out, "╔") {
		t.Errorf("lw 45 holds the title, so it keeps the box:\n%s", out)
	}
	link := "Join us: https://example.org/a/very/long/path/that/does/not/fit/inside/the/box/at/all/x"
	if out := motdBox(link, 80); strings.Contains(out, "╔") {
		t.Errorf("a word wider than the box gets the plain form:\n%s", out)
	}
}
