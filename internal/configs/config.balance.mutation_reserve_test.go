package configs

import "testing"

// #680: one mutation reserve unit as a fraction of max Conviction. Absent or
// out of range falls back to 0.11, so six units fill the 0.66 ceiling.
func TestMutationReserveUnitPctDefaults(t *testing.T) {
	for _, tc := range []struct {
		in, want float64
	}{
		{0, 0.11},
		{-1, 0.11},
		{0.9, 0.11}, // above the ceiling: no rank could ever fit
		{0.10, 0.10},
	} {
		b := Balance{PoolReservationCapPct: 0.66, MutationReserveUnitPct: ConfigFloat(tc.in)}
		b.validateSpells()
		if float64(b.MutationReserveUnitPct) != tc.want {
			t.Errorf("in %v: got %v, want %v", tc.in, b.MutationReserveUnitPct, tc.want)
		}
	}
}
