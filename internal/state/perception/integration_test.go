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

// expireCondition marks a held record spent, as the round tick does when its
// last trigger fires, without going through RemoveCondition.
func expireCondition(t *testing.T, c *characters.Character, conditionId int) {
	t.Helper()
	for _, b := range c.Conditions.List {
		if b.ConditionId == conditionId {
			b.TriggersLeft = conditions.TriggersLeftExpired
			return
		}
	}
	t.Fatalf("condition %d not held", conditionId)
}

// PE-INT-008: natural expiry. The prune pass drops a spent record and then
// calls Validate; it never calls RemoveCondition. Sight must still come back.
func TestIntegration_NaturalExpiryReturnsSighted(t *testing.T) {
	defer seedBlindConditions(t)()

	for _, id := range []int{perception.ConditionIdBlinded, perception.ConditionIdFlashbangBlindness} {
		c := characters.New()
		if err := c.AddCondition(id, false); err != nil {
			t.Fatalf("AddCondition(%d): %v", id, err)
		}
		expireCondition(t, c, id)
		c.Conditions.Prune()
		_ = c.Validate()
		if c.Perception.State() != perception.Sighted {
			t.Errorf("condition %d expired and pruned, state = %v, want Sighted", id, c.Perception.State())
		}
	}
}

// PE-INT-009: a fresh load. Perception is runtime only (yaml:"-"), so a load
// builds a Sighted machine; Validate must blind a holder whose blind
// condition is still live, or a relog cures blindness.
func TestIntegration_ReloadKeepsLiveBlindness(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if err := c.AddCondition(perception.ConditionIdBlinded, false); err != nil {
		t.Fatalf("AddCondition(3): %v", err)
	}
	c.Perception = nil // what a load from YAML hands Validate
	_ = c.Validate()
	if c.Perception.State() != perception.Blinded {
		t.Errorf("reloaded with condition 3 live, state = %v, want Blinded", c.Perception.State())
	}
}

// PE-INT-010: every door that adds a record blinds, including the
// magnitude door, which had no Perception flip of its own.
func TestIntegration_MagnitudeDoorBlinds(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if err := c.AddConditionMagnitude(perception.ConditionIdFlashbangBlindness, 0, 1, "test"); err != nil {
		t.Fatalf("AddConditionMagnitude(77): %v", err)
	}
	if c.Perception.State() != perception.Blinded {
		t.Errorf("after AddConditionMagnitude(77), state = %v, want Blinded", c.Perception.State())
	}
}
