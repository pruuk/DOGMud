package mobcommands

import (
	"slices"
	"testing"
)

func TestGearupRegistered(t *testing.T) {
	// Gearup command should be in the registered mobcommands list.
	all := GetAllMobCommands()
	found := slices.Contains(all, "gearup")
	if !found {
		t.Errorf("expected 'gearup' to be registered in mobCommands")
	}
}
