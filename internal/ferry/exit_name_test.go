package ferry

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
)

// Owner ruling 18 (2026-11-26): the ferry's boarding exit is "plank" (was
// "gangplank"), short enough for a deck button.
func TestGangplankExitName_FitsAButton(t *testing.T) {
	if gangplankExitName != "plank" {
		t.Errorf("gangplankExitName = %q, the owner ruled plank", gangplankExitName)
	}
	if err := exit.CheckName(gangplankExitName); err != nil {
		t.Error(err)
	}
}
