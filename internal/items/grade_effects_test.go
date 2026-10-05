package items

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// A graded weapon and armour piece work by their grade; standard and
// ungraded work as authored; the template is never touched.
func TestGradeEffects(t *testing.T) {
	t.Cleanup(SeedItemsForTest(map[int]*ItemSpec{
		1: {ItemId: 1, Name: "Sword", Type: Weapon, Subtype: Slashing, DamageMultiplier: 1.0, SpeedMultiplier: 0.8, Weight: 4},
		2: {ItemId: 2, Name: "Helm", Type: Head, PhysicalMitigation: 10, MagicalMitigation: 2, Weight: 4},
		3: {ItemId: 3, Name: "Pelt", Type: Object, Weight: 1},
	}))

	sword := Item{ItemId: 1, Quality: QualityPristine}
	s := sword.GetSpec()
	if !near(s.DamageMultiplier, 1.25) || !near(s.SpeedMultiplier, 0.8*1.15) || !near(s.Weight, 3.6) {
		t.Errorf("pristine sword: damage %v speed %v weight %v", s.DamageMultiplier, s.SpeedMultiplier, s.Weight)
	}
	sword.Quality = QualityCrude
	s = sword.GetSpec()
	if !near(s.DamageMultiplier, 0.85) || !near(s.SpeedMultiplier, 0.8*0.85) {
		t.Errorf("crude sword: damage %v speed %v", s.DamageMultiplier, s.SpeedMultiplier)
	}
	for _, q := range []Quality{QualityNone, QualityStandard} {
		sword.Quality = q
		if s := sword.GetSpec(); !near(s.DamageMultiplier, 1.0) || !near(s.SpeedMultiplier, 0.8) || !near(s.Weight, 4) {
			t.Errorf("%v sword should work as authored, got %+v", q, s)
		}
	}
	if raw := GetItemSpec(1); !near(raw.DamageMultiplier, 1.0) {
		t.Error("the template must never be changed")
	}

	helm := Item{ItemId: 2, Quality: QualityPristine}
	h := helm.GetSpec()
	if h.PhysicalMitigation != 13 || h.MagicalMitigation != 3 {
		t.Errorf("pristine helm: physical %d magical %d, want 13 and 3", h.PhysicalMitigation, h.MagicalMitigation)
	}
	helm.Quality = QualityCrude
	if h := helm.GetSpec(); h.PhysicalMitigation != 9 {
		t.Errorf("crude helm: physical %d, want 9", h.PhysicalMitigation)
	}

	pelt := Item{ItemId: 3, Quality: QualityPristine}
	if p := pelt.GetSpec(); !near(p.Weight, 1) {
		t.Errorf("a graded material is not gear and keeps its weight, got %v", p.Weight)
	}

	// An override is graded the same way and is not written back.
	over := Item{ItemId: 1, Quality: QualityPristine, Spec: &ItemSpec{ItemId: 1, Type: Weapon, DamageMultiplier: 2.0}}
	if s := over.GetSpec(); !near(s.DamageMultiplier, 2.5) || !near(over.Spec.DamageMultiplier, 2.0) {
		t.Errorf("override: got %v, stored %v", s.DamageMultiplier, over.Spec.DamageMultiplier)
	}
	if !near(over.GetRawSpec().DamageMultiplier, 2.0) {
		t.Error("GetRawSpec must not apply the grade")
	}
}
