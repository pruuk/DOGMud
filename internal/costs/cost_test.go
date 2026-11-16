package costs

import (
	"math"
	"testing"
)

func TestCalcComposesEveryFactor(t *testing.T) {
	// base 1.0 x encumbrance at 75% (3.1921) x skill rank 25 (1.0) x 1.25
	got := Calc(Input{
		Base: 1.0, Carried: 75, Capacity: 100, Physical: true,
		SkillRank: 25, HasSkill: true, Modifier: 1.25,
	})
	if math.Abs(got-3.9902) > 0.001 {
		t.Fatalf("got %.4f, want 3.9902", got)
	}
}

// Mental and social actions never take encumbrance, however laden the actor is.
func TestCalcSkipsEncumbranceForNonPhysical(t *testing.T) {
	laden := Input{Base: 10, Carried: 100, Capacity: 100, Physical: false,
		SkillRank: 25, HasSkill: true, Modifier: 1.0}
	empty := laden
	empty.Carried = 0

	if Calc(laden) != Calc(empty) {
		t.Fatalf("a mental action must cost the same laden or empty")
	}
}

// An action with no associated skill takes no skill multiplier.
func TestCalcSkipsSkillWhenTheActionHasNone(t *testing.T) {
	in := Input{Base: 10, Physical: false, HasSkill: false, SkillRank: 0, Modifier: 1.0}
	if math.Abs(Calc(in)-10.0) > 0.001 {
		t.Fatalf("got %.4f, want 10.0", Calc(in))
	}
}

// The PRODUCT is clamped, not each factor.
func TestCalcClampsTheProduct(t *testing.T) {
	got := Calc(Input{
		Base: 1.0, Carried: 100, Capacity: 100, Physical: true,
		SkillRank: 0, HasSkill: true, Modifier: 1.25,
	})
	// unclamped this is 5.5 x 1.10 x 1.25 = 7.5625, over the Go default cap of 7.5
	if got > 7.5001 {
		t.Fatalf("product not clamped: got %.4f, want <= 7.5", got)
	}
}

// A realistic character's real numbers, so the bench and the code cannot drift.
func TestCalcMatchesTheTunedBenchFigures(t *testing.T) {
	cases := []struct {
		name              string
		carried, capacity float64
		rank              int
		wantSwing         float64
	}{
		// Re-derived for the smooth curve (#401): encumbrance x skill.
		{"newcomer", 35.2, 100, 1, 1.459}, // 1.3308 x 1.096
		{"mid", 44.9, 100, 25, 1.608},     // 1.6080 x 1.000
		{"meirok", 36.7, 100, 69, 0.886},  // 1.3672 x 0.648
	}
	for _, c := range cases {
		got := Calc(Input{
			Base: 1.0, Carried: c.carried, Capacity: c.capacity, Physical: true,
			SkillRank: c.rank, HasSkill: true, Modifier: 1.0,
		})
		if math.Abs(got-c.wantSwing) > 0.01 {
			t.Errorf("%s: swing cost %.3f, want %.2f", c.name, got, c.wantSwing)
		}
	}
}

func TestSpecForUnregisteredActionIsInert(t *testing.T) {
	s := SpecFor(Action("not-a-real-action"))
	if s.SkillSource != SkillNone || s.Physical {
		t.Fatalf("an unregistered action must price as a flat base, got %+v", s)
	}
}
