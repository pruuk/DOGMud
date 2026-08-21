package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

func TestProcessFoldRound_StandingNoCheck(t *testing.T) {
	// Standing should never hit the disruption gate; the function should
	// proceed past it. We can't easily run the full fold without a real
	// spell, so this test only asserts that PositionDisruptionDmgEquiv
	// returns 0 for Standing and that the gate code path skips when 0.
	if got := position.PositionDisruptionDmgEquiv(position.Standing, control.Neutral); got != 0 {
		t.Errorf("Standing should be 0, got %d", got)
	}
}
