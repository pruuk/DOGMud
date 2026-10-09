package mobcommands

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// #449: the mob throttle's player-facing lines ride the move's own category
// (hit) or the disruption category (cast interrupt), never CategorySystem,
// which never wraps. Source-level pin.
func TestMobThrottleNarrationNeverRidesSystem(t *testing.T) {
	raw, err := os.ReadFile("throttle.go")
	require.NoError(t, err)
	src := string(raw)
	require.NotContains(t, src, "CategorySystem")
	require.Contains(t, src, `sendMoveEvent("throttle", "hit", ids, aud, messaging.CategoryHitNaturalSharp`)
	require.Contains(t, src, `sendMoveEvent("throttle", "cast_interrupt", ids, aud, messaging.CategorySpellDisruption`)
}
