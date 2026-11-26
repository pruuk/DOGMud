package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
)

// Owner ruling 18 (2026-11-26): a mob's portal is the "rift" exit (was
// "dark portal"), short enough for a deck button.
func TestMobPortalExitName_FitsAButton(t *testing.T) {
	if mobPortalExitName != "rift" {
		t.Errorf("mobPortalExitName = %q, the owner ruled rift", mobPortalExitName)
	}
	if err := exit.CheckName(mobPortalExitName); err != nil {
		t.Error(err)
	}
}
