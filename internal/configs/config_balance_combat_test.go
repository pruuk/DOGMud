package configs

import "testing"

func TestU10dKnobs_DefaultAndValidate(t *testing.T) {
	cases := []struct {
		name string
		get  func(*Balance) float64
		set  func(*Balance, float64)
	}{
		{"SurpriseOpeningStrikeMultiplier",
			func(b *Balance) float64 { return float64(b.SurpriseOpeningStrikeMultiplier) },
			func(b *Balance, v float64) { b.SurpriseOpeningStrikeMultiplier = ConfigFloat(v) }},
		{"SurpriseRangedStrikeMultiplier",
			func(b *Balance) float64 { return float64(b.SurpriseRangedStrikeMultiplier) },
			func(b *Balance, v float64) { b.SurpriseRangedStrikeMultiplier = ConfigFloat(v) }},
		{"RangedUnengagedDamageMultiplier",
			func(b *Balance) float64 { return float64(b.RangedUnengagedDamageMultiplier) },
			func(b *Balance, v float64) { b.RangedUnengagedDamageMultiplier = ConfigFloat(v) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b Balance

			// Absent (zero) must default to 1.0, NOT stay at 0.
			tc.set(&b, 0)
			b.Validate()
			if got := tc.get(&b); got != 1.0 {
				t.Fatalf("zero must default to 1.0, got %v", got)
			}

			tc.set(&b, -3)
			b.Validate()
			if got := tc.get(&b); got != 1.0 {
				t.Fatalf("negative must reset to 1.0, got %v", got)
			}

			tc.set(&b, 2.5)
			b.Validate()
			if got := tc.get(&b); got != 2.5 {
				t.Fatalf("a legal value must survive validation, got %v", got)
			}
		})
	}
}
