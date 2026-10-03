package perception_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
)

// seedBlindConditions registers minimal ConditionSpec entries for the two blind-source
// condition IDs (3 = Blinded, 77 = Flashbang Blindness) into the global condition
// registry and returns a cleanup function that restores the original state.
//
// Without seeded specs, conditions.AddCondition returns false (spec not found) and
// characters.AddCondition returns an error, causing every PE-INT test to fail
// at setup rather than at the assertion under test.
//
// TriggerCount=5, RoundInterval=1 gives a non-zero lifespan so the condition
// is not immediately expired on add.
func seedBlindConditions(t *testing.T) func() {
	t.Helper()
	cleanup := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		perception.ConditionIdBlinded: {
			ConditionId:   perception.ConditionIdBlinded,
			Name:          "Blinded",
			Description:   "Your eyes are blinded.",
			TriggerCount:  5,
			RoundInterval: 1,
		},
		perception.ConditionIdFlashbangBlindness: {
			ConditionId:   perception.ConditionIdFlashbangBlindness,
			Name:          "Flashbang Blindness",
			Description:   "A flash of light sears your vision.",
			TriggerCount:  3,
			RoundInterval: 1,
		},
	})
	return cleanup
}

// TestMain initialises the mudlog logger once for all integration tests in
// this file. conditions.Validate() calls mudlog.Warn when it encounters an unknown
// conditionId; without initialisation the logger panics on a nil receiver.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	m.Run()
}

// PE-INT-001: AddCondition(3) → Blinded.
func TestIntegration_ConditionBlindedAppliesBlinded(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if c.Perception.State() != perception.Sighted {
		t.Fatalf("initial state = %v, want Sighted", c.Perception.State())
	}
	if err := c.AddCondition(perception.ConditionIdBlinded, false); err != nil {
		t.Fatalf("AddCondition(3): %v", err)
	}
	if c.Perception.State() != perception.Blinded {
		t.Errorf("after AddCondition(3), state = %v, want Blinded", c.Perception.State())
	}
}

// PE-INT-002: AddCondition(3) + AddCondition(77) + RemoveCondition(3) → still Blinded.
//
// The second source used to be the ConditionBlinded enum entry. The enum is
// gone and conditions 3 and 77 are the two blind sources that remain, so the
// overlap this test exists to pin is now condition-on-condition.
func TestIntegration_OverlapKeepsBlinded(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if err := c.AddCondition(perception.ConditionIdBlinded, false); err != nil {
		t.Fatalf("AddCondition(3): %v", err)
	}
	if err := c.AddCondition(perception.ConditionIdFlashbangBlindness, false); err != nil {
		t.Fatalf("AddCondition(77): %v", err)
	}
	c.RemoveCondition(perception.ConditionIdBlinded)
	if c.Perception.State() != perception.Blinded {
		t.Errorf("after removing condition 3 but condition 77 still active, state = %v, want Blinded", c.Perception.State())
	}
}

// PE-INT-003: Add both, remove both → Sighted.
func TestIntegration_AllSourcesClearedReturnsSighted(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	_ = c.AddCondition(perception.ConditionIdBlinded, false)
	_ = c.AddCondition(perception.ConditionIdFlashbangBlindness, false)
	c.RemoveCondition(perception.ConditionIdBlinded)
	c.RemoveCondition(perception.ConditionIdFlashbangBlindness)
	if c.Perception.State() != perception.Sighted {
		t.Errorf("after clearing all sources, state = %v, want Sighted", c.Perception.State())
	}
}

// PE-INT-004 (PE-010 from spec matrix): re-applying condition while already
// Blinded is a no-op (no ErrInvalidTransition propagated, no log spam).
func TestIntegration_ReapplyConditionNoOp(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if err := c.AddCondition(perception.ConditionIdBlinded, false); err != nil {
		t.Fatalf("first AddCondition: %v", err)
	}
	// Re-add the same condition (the condition system stacks duration, but the
	// blind-source state is the same). The current-state guard in
	// AddCondition prevents the transition from firing twice.
	if err := c.AddCondition(perception.ConditionIdBlinded, false); err != nil {
		t.Fatalf("second AddCondition: %v", err)
	}
	if c.Perception.State() != perception.Blinded {
		t.Errorf("after duplicate AddCondition, state = %v, want Blinded", c.Perception.State())
	}
}

// PE-INT-005: Flashbang (condition 77) drives the same Perception transitions.
func TestIntegration_FlashbangBlindness(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if err := c.AddCondition(perception.ConditionIdFlashbangBlindness, false); err != nil {
		t.Fatalf("AddCondition(77): %v", err)
	}
	if c.Perception.State() != perception.Blinded {
		t.Errorf("after AddCondition(77), state = %v, want Blinded", c.Perception.State())
	}
	c.RemoveCondition(perception.ConditionIdFlashbangBlindness)
	if c.Perception.State() != perception.Sighted {
		t.Errorf("after RemoveCondition(77), state = %v, want Sighted", c.Perception.State())
	}
}

// PE-INT-007: Mixed source order — flashbang first, then condition 3, then the
// flashbang removed → still Blinded (condition 3 still active).
func TestIntegration_MixedSourceOrder(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if err := c.AddCondition(perception.ConditionIdFlashbangBlindness, false); err != nil {
		t.Fatalf("AddCondition(77): %v", err)
	}
	if c.Perception.State() != perception.Blinded {
		t.Fatalf("after AddCondition(77), state = %v, want Blinded", c.Perception.State())
	}
	if err := c.AddCondition(perception.ConditionIdBlinded, false); err != nil {
		t.Fatalf("AddCondition(3): %v", err)
	}
	// Re-adding-while-already-Blinded path; state must remain Blinded.
	if c.Perception.State() != perception.Blinded {
		t.Errorf("after AddCondition(3) while already blinded, state = %v, want Blinded", c.Perception.State())
	}
	c.RemoveCondition(perception.ConditionIdFlashbangBlindness)
	// Condition 3 still active → still Blinded.
	if c.Perception.State() != perception.Blinded {
		t.Errorf("after RemoveCondition(77) (condition 3 still active), state = %v, want Blinded", c.Perception.State())
	}
	c.RemoveCondition(perception.ConditionIdBlinded)
	if c.Perception.State() != perception.Sighted {
		t.Errorf("after RemoveCondition (no sources left), state = %v, want Sighted", c.Perception.State())
	}
}

// PE-INT-RIFT: a blind condition that simply runs out (triggered to zero and
// pruned on the turn tick, never passing through RemoveCondition) returns the
// character to Sighted once Validate runs, as every prune path does. Before
// the fix the machine stayed Blinded until something reset it.
func TestIntegration_ExpiredByPruneReturnsSighted(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if err := c.AddCondition(perception.ConditionIdBlinded, false); err != nil {
		t.Fatalf("AddCondition(3): %v", err)
	}
	if c.Perception.State() != perception.Blinded {
		t.Fatalf("after AddCondition: state = %v, want Blinded", c.Perception.State())
	}
	for i := 0; i < 20 && c.Conditions.TriggersLeft(perception.ConditionIdBlinded) > 0; i++ {
		c.Conditions.Trigger()
	}
	if pruned := c.Conditions.Prune(); len(pruned) == 0 {
		t.Fatalf("expected the expired blind condition to be pruned")
	}
	_ = c.Validate()
	if c.Perception.State() != perception.Sighted {
		t.Fatalf("after prune + Validate: state = %v, want Sighted", c.Perception.State())
	}
}
