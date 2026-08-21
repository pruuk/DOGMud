package combat

import "testing"

// The critical message pool and the *** crit banner must agree. Bands are
// chosen by pctDamage (actual vs expected mean), where >= 101 selects the
// crit-worded pool. Before this clamp, roughly half of ordinary hits
// over-rolled the mean and printed crit-worded lines bare (the U6b flip
// re-check's "quadruple fumble-crit line" finding), while a heavily
// mitigated real crit could banner weak-worded text.
func TestAttackMessagePct_CritAndWordingAgree(t *testing.T) {
	tests := []struct {
		name string
		pct  int
		crit bool
		want int
	}{
		{"non-crit over-roll clamps to heavy band", 140, false, 100},
		{"non-crit exactly at crit threshold clamps", 101, false, 100},
		{"non-crit at mean stays", 100, false, 100},
		{"non-crit heavy stays", 80, false, 80},
		{"non-crit normal stays", 50, false, 50},
		{"non-crit weak stays", 10, false, 10},
		{"non-crit zero stays (miss band, feint gate)", 0, false, 0},
		{"crit below threshold promotes to crit band", 40, true, 101},
		{"crit at zero promotes to crit band", 0, true, 101},
		{"crit over threshold stays", 150, true, 150},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := attackMessagePct(tc.pct, tc.crit); got != tc.want {
				t.Fatalf("attackMessagePct(%d, %v) = %d, want %d", tc.pct, tc.crit, got, tc.want)
			}
		})
	}
}
