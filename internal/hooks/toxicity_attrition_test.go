package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

// Toxicity sickness hurts, but it is attrition, not an attack: a craft in hand
// keeps running (#514 R2), and nothing is queued for the round-end flush.
func TestTickToxicity_SicknessLeavesACraftRunning(t *testing.T) {
	takeWorkInterrupts()
	t.Cleanup(func() { takeWorkInterrupts() })

	c := characters.New()
	c.HealthMax.Value = 100
	c.Health = 100
	// Well past the 90% sickness line even after this tick's decay.
	c.Toxicity = c.GetToxicityMax() * 2

	err := c.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: "toxic-recipe", RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}

	tickToxicity(c)

	if c.Health >= 100 {
		t.Fatalf("health is %d after the tick, want sickness damage", c.Health)
	}
	if st := c.Activity.State(); st != activity.Crafting {
		t.Fatalf("activity is %v after the tick, want Crafting", st)
	}
	if pending := takeWorkInterrupts(); len(pending) != 0 {
		t.Fatalf("toxicity queued %d interrupt lines, want none", len(pending))
	}
}
