package buffs

import "testing"

// TestTickTriggers pins the rounds-to-triggers arithmetic that keeps a
// triggerrate-3-rounds record (Poisoned, Bleeding) dealing the same total
// damage the old AutoHeal-gated conditions did: AutoHeal only ever applied
// its DoT on every third round while duration counted down every round, so
// a duration of rounds produced rounds/3 ticks, floored to at least one.
func TestTickTriggers(t *testing.T) {
	cases := []struct {
		rounds   int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 1},
		{3, 1},
		{4, 1},
		{5, 1},
		{6, 2},
		{20, 6},
	}
	for _, c := range cases {
		if got := TickTriggers(c.rounds); got != c.expected {
			t.Errorf("TickTriggers(%d) = %d, want %d", c.rounds, got, c.expected)
		}
	}
}
