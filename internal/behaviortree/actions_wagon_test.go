package behaviortree

import (
	"testing"
)

func TestActDistributeCargoToHostiles_Registered(t *testing.T) {
	if _, ok := actionRegistry["distribute_cargo_to_hostiles"]; !ok {
		t.Error("distribute_cargo_to_hostiles not registered in actionRegistry")
	}
}
