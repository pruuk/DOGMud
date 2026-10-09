package messaging

import (
	"strings"
	"testing"
)

// PIN, not a red-first test: the crit banner (combat.critBannerClose, #455)
// relies on WrapAnsi breaking only at an ASCII space, so a no-break space
// (U+00A0) keeps the closing *** with the last word at every width.
func TestWrapAnsi_NeverBreaksAtANoBreakSpace(t *testing.T) {
	nbsp := string(rune(0x00A0))
	in := `<ansi fg="crit-text">***</ansi> You strike Early Strider!` + nbsp + `<ansi fg="crit-text">***</ansi>`
	for width := 10; width <= 60; width++ {
		lines := strings.Split(WrapAnsi(in, width), "\n")
		last := lines[len(lines)-1]
		if !strings.Contains(last, "Strider!"+nbsp) {
			t.Errorf("width %d: the closing *** left its last word: %q", width, lines)
		}
	}
}
