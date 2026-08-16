package templates

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderReservation runs the real template engine over the reservationQuality
// action rather than calling the closure directly. Executing the action is the
// point: a typo in the registered name, an arity mismatch or a bad argument
// type all surface here and nowhere else short of booting the server and
// typing `status`.
func renderReservation(t *testing.T, band string) string {
	t.Helper()
	out, err := ProcessText(
		`{{ reservationQuality .Band 13 }}`,
		map[string]any{"Band": band},
	)
	require.NoError(t, err)
	return strings.TrimRight(out, "\r\n")
}

// TestReservationQuality_RendersTheWordAndPadsLikeItsSiblings asserts the
// contract the status sheet depends on. That sheet is a fixed 80-column box
// drawn with box characters, and every value cell is padded to 13 visible
// characters, so a cell that under- or over-pads visibly breaks the right-hand
// border on every `status` for every player.
func TestReservationQuality_RendersTheWordAndPadsLikeItsSiblings(t *testing.T) {
	tests := []struct {
		band  string
		color string
	}{
		{"none", `fg="green"`},
		{"slight", `fg="green"`},
		{"modest", `fg="green"`},
		{"notable", `fg="yellow"`},
		{"heavy", `fg="red"`},
		{"at limit", `fg="red-bold"`},
	}

	for _, tt := range tests {
		t.Run(tt.band, func(t *testing.T) {
			out := renderReservation(t, tt.band)

			assert.Contains(t, out, tt.band)

			// Colour escalates with severity. "at limit" in particular must be
			// the loudest of them: it is the only band a player can act on.
			assert.Contains(t, out, tt.color,
				"band %q must render in %s, got %q", tt.band, tt.color, out)

			// Reservation is disclosed in words only. A digit here would leak a
			// balance value straight onto the status sheet.
			assert.False(t, strings.ContainsAny(out, "0123456789"),
				"rendered band %q contains a digit: %q", tt.band, out)

			// Measure what the player's terminal occupies, not what the template
			// emitted: the ansi markup is resolved downstream and takes up no
			// columns.
			visible := ansiTagStripper.ReplaceAllString(out, "")
			assert.Equal(t, 13, len([]rune(visible)),
				"the cell must occupy exactly 13 visible columns, got %q", visible)
		})
	}
}

// TestReservationQuality_RendersWhatTheCharacterPackageActuallyReturns closes
// the loop between the two packages. The band words above are literals, so on
// their own they cannot catch a rename in characters.reservationBand; driving
// real reservation totals through Character.ReservationBandName and into the
// renderer can, and it also proves the status template's argument shape
// (a pool name string) is the one the method really takes.
//
// Conviction is used because a companion's reserve is settable without building
// an equipment set, and the cap is floor(max x PoolReservationCapPct).
func TestReservationQuality_RendersWhatTheCharacterPackageActuallyReturns(t *testing.T) {
	tests := []struct {
		name    string
		reserve int
		want    string
	}{
		{"nothing bound", 0, "none"},
		{"one small bond", 100, "slight"},
		{"a real cost", 200, "modest"},
		{"giving something up", 400, "notable"},
		{"close to the ceiling", 600, "heavy"},
		{"ceiling reached", 660, "at limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &characters.Character{}
			c.ConvictionMax.Value = 1000
			if tt.reserve > 0 {
				c.Companions = []characters.CompanionInfo{
					{ConvictionReserve: tt.reserve},
				}
			}

			band := c.ReservationBandName("conviction")
			assert.Equal(t, tt.want, band)

			visible := ansiTagStripper.ReplaceAllString(renderReservation(t, band), "")
			assert.Equal(t, 13, len([]rune(visible)),
				"band %q from the characters package does not fit the status column: %q",
				band, visible)
		})
	}
}
