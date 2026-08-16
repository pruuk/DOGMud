package templates

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ansiTagStripper removes the <ansi ...> markup so a rendered cell can be
// measured in the columns it will actually occupy on the player's screen.
var ansiTagStripper = regexp.MustCompile(`</?ansi[^>]*>`)

// TestEncumbranceQuality_RendersTheWordAndPadsLikeItsSiblings exercises the
// template function the `status` sheet now calls, through the real template
// engine rather than by calling the closure directly. Executing the actual
// template action is the point: a typo in the func name, an arity mismatch or
// a bad argument type all surface here and nowhere else short of booting the
// server and typing `status`.
//
// Padding is asserted because the status sheet is a fixed 80-column box drawn
// with box characters. Every value cell is padded to 13 visible characters, so
// a cell that under- or over-pads visibly breaks the right-hand border.
func TestEncumbranceQuality_RendersTheWordAndPadsLikeItsSiblings(t *testing.T) {
	tests := []struct {
		name     string
		carried  float64
		capacity float64
		want     string
	}{
		{"empty pack", 0, 100, "light"},
		{"quarter", 25, 100, "light"},
		{"half", 50, 100, "moderate"},
		{"three quarters", 75, 100, "heavy"},
		{"at capacity", 100, 100, "overburdened"},
		{"over capacity", 150, 100, "crushed"},
		// A capacity of zero must not divide by zero; "crushed" is the honest
		// reading for an actor who cannot carry anything at all.
		{"no capacity", 10, 0, "crushed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := ProcessText(
				`{{ encumbranceQuality .Carried .Capacity 13 }}`,
				map[string]any{"Carried": tt.carried, "Capacity": tt.capacity},
			)
			require.NoError(t, err)

			assert.Contains(t, out, tt.want)

			// No raw numbers reach the player from this cell, ever: carried
			// weight prices every physical action now, which makes it a balance
			// value.
			assert.NotContains(t, out, "100")

			// Measure what the player's terminal occupies, not what the
			// template emitted: the ansi markup is resolved downstream and
			// takes up no columns.
			visible := ansiTagStripper.ReplaceAllString(strings.TrimRight(out, "\r\n"), "")
			assert.Equal(t, 13, len([]rune(visible)),
				"the cell must occupy exactly 13 visible columns, got %q", visible)
		})
	}
}
