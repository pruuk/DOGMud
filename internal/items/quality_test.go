package items

import (
	"strings"
	"testing"
)

func TestQuality_NamesRoundTrip(t *testing.T) {
	for q := QualityMin; q <= QualityMax; q++ {
		name := q.String()
		if name == `` {
			t.Fatalf("grade %d has no name", q)
		}
		back, ok := ParseQuality(name)
		if !ok || back != q {
			t.Errorf("ParseQuality(%q) = %v,%v; want %v", name, back, ok, q)
		}
	}
	if QualityNone.String() != `` || QualityNone.Valid() {
		t.Error("QualityNone must be nameless and not a valid grade")
	}
}

func TestQuality_ClampLeavesUngradedAlone(t *testing.T) {
	if QualityNone.Clamp() != QualityNone {
		t.Error("clamping an ungraded item must not grade it")
	}
	if Quality(-3).Clamp() != QualityCrude {
		t.Error("below range clamps to crude")
	}
	if Quality(9).Clamp() != QualityPristine {
		t.Error("above range clamps to pristine")
	}
}

// Ungraded items must price exactly as before grading existed.
func TestQualityValueMultiplier_UngradedIsNeutral(t *testing.T) {
	if got := QualityValueMultiplier(QualityNone); got != 1.0 {
		t.Errorf("ungraded multiplier = %v, want 1.0", got)
	}
	if QualityValueMultiplier(QualityCrude) >= QualityValueMultiplier(QualityStandard) {
		t.Error("crude must sell below standard")
	}
	if QualityValueMultiplier(QualityPristine) <= QualityValueMultiplier(QualitySuperb) {
		t.Error("pristine must sell above superb")
	}
}

func TestQuality_SuffixShowsNotableGradesOnly(t *testing.T) {
	if QualityNone.qualitySuffix() != `` || QualityStandard.qualitySuffix() != `` {
		t.Error("ungraded and standard print no suffix")
	}
	for _, q := range []Quality{QualityCrude, QualityFine, QualitySuperb, QualityPristine} {
		if !strings.Contains(q.qualitySuffix(), q.String()) {
			t.Errorf("grade %v suffix %q should name the grade", q, q.qualitySuffix())
		}
	}
}

func TestSameStack_SeparatesGrades(t *testing.T) {
	a := Item{ItemId: 40002, Quality: QualityFine}
	b := Item{ItemId: 40002, Quality: QualityCrude}
	if SameStack(a, b) {
		t.Error("a fine and a crude strip must not share a stack")
	}
	c := Item{ItemId: 40002}
	if SameStack(a, c) {
		t.Error("a graded and an ungraded strip must not share a stack")
	}
	if !SameStack(a, Item{ItemId: 40002, Quality: QualityFine}) {
		t.Error("two fine strips stack")
	}
}
