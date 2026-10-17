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
	text := "Welcome to Delusions of Grandeur. Read the rules, mind the " +
		"newbie zone hazards, and ask a mod if anything looks broken."
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
