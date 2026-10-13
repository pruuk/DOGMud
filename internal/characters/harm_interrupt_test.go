package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

func craftingChar(t *testing.T) (*Character, *[]string) {
	t.Helper()
	c := poolChar(50, 50, 50)
	triggers := &[]string{}
	c.Activity.OnCraftEnded("t", func(_ activity.CraftingData, r state.TransitionReason) {
		*triggers = append(*triggers, r.Trigger)
	})
	err := c.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: "r1", RoundsTotal: 3},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	return c, triggers
}

func TestApplyHarm_HealthDamageInterruptsCraft(t *testing.T) {
	c, triggers := craftingChar(t)
	c.ApplyHarm(PoolHealth, 5, state.ActorRef{})
	if c.Activity.State() != activity.Free {
		t.Fatalf("state = %v, want Free after health harm", c.Activity.State())
	}
	if len(*triggers) != 1 || (*triggers)[0] != activity.TriggerDamageInterrupt {
		t.Errorf("craft-ended triggers = %v, want one damage interrupt", *triggers)
	}
}

func TestApplyHarm_HealthDamageInterruptsSalvage(t *testing.T) {
	c := poolChar(50, 50, 50)
	err := c.Activity.TransitionToSalvaging(
		activity.SalvagingData{ItemUuid: "u", RoundsTotal: 3},
		state.TransitionReason{Trigger: activity.TriggerSalvageBegin},
	)
	if err != nil {
		t.Fatalf("begin salvage: %v", err)
	}
	c.ApplyHarm(PoolHealth, 5, state.ActorRef{})
	if c.Activity.State() != activity.Free {
		t.Fatalf("state = %v, want Free after health harm", c.Activity.State())
	}
}

func TestApplyAttrition_DoesNotInterruptButHurtsTheSame(t *testing.T) {
	c, triggers := craftingChar(t)
	got := c.ApplyAttrition(PoolHealth, 5, state.ActorRef{})
	if got != 5 || c.Health != 45 {
		t.Errorf("applied %d, health %d; want 5 and 45", got, c.Health)
	}
	if c.Activity.State() != activity.Crafting {
		t.Errorf("state = %v, want Crafting after attrition", c.Activity.State())
	}
	if len(*triggers) != 0 {
		t.Errorf("craft-ended fired: %v", *triggers)
	}
}

func TestApplyHarm_ConvictionDoesNotInterrupt(t *testing.T) {
	c, _ := craftingChar(t)
	c.ApplyHarm(PoolConviction, 5, state.ActorRef{})
	if c.Activity.State() != activity.Crafting {
		t.Errorf("state = %v, want Crafting after conviction harm", c.Activity.State())
	}
}

func TestApplyHarm_ZeroDoesNotInterrupt(t *testing.T) {
	c, _ := craftingChar(t)
	c.ApplyHarm(PoolHealth, 0, state.ActorRef{})
	if c.Activity.State() != activity.Crafting {
		t.Errorf("state = %v, want Crafting after zero harm", c.Activity.State())
	}
}
