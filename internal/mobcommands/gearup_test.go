package mobcommands

import (
	"testing"
)

func TestGearupRegistered(t *testing.T) {
	// Gearup command should be in the registered mobcommands list.
	all := GetAllMobCommands()
	found := false
	for _, cmd := range all {
		if cmd == "gearup" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'gearup' to be registered in mobCommands")
	}
}
