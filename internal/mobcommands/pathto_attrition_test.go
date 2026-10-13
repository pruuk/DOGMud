package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

// A mob stuck away from home decays, but the decay is attrition, not an
// attack: a craft in hand keeps running (#514 R2).
func TestPathtoHomeDecay_LeavesACraftRunning(t *testing.T) {
	m := &mobs.Mob{InstanceId: 98471}
	m.Character.Name = "Lost Smith"
	m.Character.MobInstanceId = 98471
	m.Character.Validate() // builds the Activity machine
	m.Character.HealthMax.Value = 100
	m.Character.Health = 100
	m.SetTempData(`home-impossible`, true)

	err := m.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: "stuck-mob-recipe", RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}

	if ok, err := Pathto(`home`, m, nil); !ok || err != nil {
		t.Fatalf("Pathto home = %v, %v; want true, nil", ok, err)
	}

	if m.Character.Health != 90 {
		t.Fatalf("health is %d after the decay, want 90", m.Character.Health)
	}
	if st := m.Character.Activity.State(); st != activity.Crafting {
		t.Fatalf("activity is %v after the decay, want Crafting", st)
	}
}
