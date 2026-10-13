package activity_test

import (
	"errors"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

type craftEndedCall struct {
	data    activity.CraftingData
	trigger string
}

func craftEndedRecorder(m *activity.Machine) *[]craftEndedCall {
	calls := &[]craftEndedCall{}
	m.OnCraftEnded("t", func(d activity.CraftingData, r state.TransitionReason) {
		*calls = append(*calls, craftEndedCall{d, r.Trigger})
	})
	return calls
}

func beginCraft(t *testing.T, m *activity.Machine) {
	t.Helper()
	err := m.TransitionToCrafting(
		activity.CraftingData{RecipeId: "r1", RoundsTotal: 3},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
}

func TestCraftEnded_FiresOnceOnFree(t *testing.T) {
	m := activity.NewMachine()
	calls := craftEndedRecorder(m)
	beginCraft(t, m)
	if err := m.TransitionToFree(state.TransitionReason{Trigger: activity.TriggerCraftCancel}); err != nil {
		t.Fatalf("free: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("handler calls = %d, want 1", len(*calls))
	}
	got := (*calls)[0]
	if got.data.RecipeId != "r1" || got.trigger != activity.TriggerCraftCancel {
		t.Errorf("got recipe %q trigger %q", got.data.RecipeId, got.trigger)
	}
}

func TestCraftEnded_FiresOnForceFree(t *testing.T) {
	m := activity.NewMachine()
	calls := craftEndedRecorder(m)
	beginCraft(t, m)
	m.ForceFree(state.TransitionReason{Trigger: activity.TriggerDamageInterrupt})
	if len(*calls) != 1 {
		t.Fatalf("handler calls = %d, want 1", len(*calls))
	}
	if (*calls)[0].trigger != activity.TriggerDamageInterrupt || (*calls)[0].data.RecipeId != "r1" {
		t.Errorf("got %+v", (*calls)[0])
	}
}

func TestCraftEnded_NotForCastingOrSalvaging(t *testing.T) {
	m := activity.NewMachine()
	calls := craftEndedRecorder(m)
	_ = m.TransitionToCasting(activity.CastingData{SpellId: "s"},
		state.TransitionReason{Trigger: activity.TriggerCastBegin})
	_ = m.TransitionToFree(state.TransitionReason{Trigger: activity.TriggerCastCancel})
	_ = m.TransitionToSalvaging(activity.SalvagingData{ItemUuid: "u"},
		state.TransitionReason{Trigger: activity.TriggerSalvageBegin})
	_ = m.TransitionToFree(state.TransitionReason{Trigger: activity.TriggerSalvageCancel})
	m.ForceFree(state.TransitionReason{Trigger: activity.TriggerDeath})
	if len(*calls) != 0 {
		t.Errorf("handler calls = %d, want 0", len(*calls))
	}
}

func TestCraftEnded_NotWhenTheTransitionIsVetoed(t *testing.T) {
	m := activity.NewMachine()
	calls := craftEndedRecorder(m)
	m.Inner().BeforeTransition("refuse_free", func(from, to activity.State, r state.TransitionReason) error {
		if to == activity.Free {
			return errors.New("refused")
		}
		return nil
	})
	beginCraft(t, m)
	if err := m.TransitionToFree(state.TransitionReason{Trigger: activity.TriggerCraftCancel}); err == nil {
		t.Fatal("expected the veto to refuse the transition")
	}
	if len(*calls) != 0 {
		t.Errorf("handler calls = %d, want 0 after a vetoed transition", len(*calls))
	}
	if !m.IsCrafting() {
		t.Errorf("state = %v, want Crafting after a vetoed transition", m.State())
	}
}

func TestCraftEnded_NotWhenAlreadyFree(t *testing.T) {
	m := activity.NewMachine()
	calls := craftEndedRecorder(m)
	// Free to Free is not in the transition table, so it returns an error.
	if err := m.TransitionToFree(state.TransitionReason{Trigger: activity.TriggerCraftCancel}); err == nil {
		t.Errorf("expected an error freeing an already-free machine")
	}
	m.ForceFree(state.TransitionReason{Trigger: activity.TriggerCraftCancel})
	if len(*calls) != 0 {
		t.Errorf("handler calls = %d, want 0", len(*calls))
	}
}
