package characters

import "testing"

func TestDisplayHealth_ClampsNegativeToZero(t *testing.T) {
	c := New()
	c.Health = -25
	if got := c.DisplayHealth(); got != 0 {
		t.Errorf("DisplayHealth() with Health=-25 = %d, want 0", got)
	}
}

func TestDisplayHealth_PassesThroughNonNegative(t *testing.T) {
	c := New()
	for _, hp := range []int{0, 1, 137} {
		c.Health = hp
		if got := c.DisplayHealth(); got != hp {
			t.Errorf("DisplayHealth() with Health=%d = %d, want %d", hp, got, hp)
		}
	}
}
