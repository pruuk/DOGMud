package playtestprofiles

import "testing"

// Regression: overlays.grant_items could not grant ANY item with non-zero
// weight, and the failure killed the container at boot rather than surfacing.
//
// StoreItem refuses a store that would push carried weight past
// CarryCapacity()*2, and CarryCapacity reads Stats.Strength.ValueAdj. Profile
// YAML sets only .Base -- ValueAdj is DERIVED. Materialize used to call
// ApplyOverlays before Validate(true), so ValueAdj was still 0, capacity was
// 0.0, and every weighted item was rejected with "could not store item N".
//
// Measured on the early profile before the fix: Str.Base=105, ValueAdj=0,
// capacity 0.0. After deriving: ValueAdj=105, capacity 68.2.
//
// This pins the ordering contract Materialize now depends on. It uses the real
// template loader deliberately: a characters.New() fixture already carries
// derived default stats and cannot reproduce the bug.
func TestLoadedTemplate_HasNoDerivedStatsUntilRecalculated(t *testing.T) {
	u, err := LoadTemplate("../../tools/playtest/profiles", "early")
	if err != nil {
		t.Fatalf("load early template: %v", err)
	}
	c := u.Character

	if c.Stats.Strength.Base != 105 {
		t.Fatalf("early profile Strength.Base = %d, want 105 (profile changed; update this test)", c.Stats.Strength.Base)
	}
	if c.Stats.Strength.ValueAdj != 0 {
		t.Fatalf("Strength.ValueAdj = %d straight off the loader, want 0 (derived, not loaded)", c.Stats.Strength.ValueAdj)
	}
	if got := c.CarryCapacity(); got != 0 {
		t.Fatalf("CarryCapacity with underived stats = %.1f, want 0 -- this is what rejected every weighted grant_item", got)
	}

	c.RecalculateStats()

	if c.Stats.Strength.ValueAdj != 105 {
		t.Errorf("after RecalculateStats, Strength.ValueAdj = %d, want 105", c.Stats.Strength.ValueAdj)
	}
	if got := c.CarryCapacity(); got <= 0 {
		t.Errorf("after RecalculateStats, CarryCapacity = %.1f, want > 0", got)
	}
}
