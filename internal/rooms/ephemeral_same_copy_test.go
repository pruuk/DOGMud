package rooms

import "testing"

// Two rooms are in one ephemeral copy only when both are ephemeral and share
// a chunk (one CreateEphemeralRoomIds call, one zone instance).
func TestSameEphemeralCopy(t *testing.T) {
	base := ephemeralRoomIdMinimum
	cases := []struct {
		a, b int
		want bool
	}{
		{base + 3, base + 7, true},
		{base, base + ephemeralChunkSize - 1, true},
		{base + 3, base + ephemeralChunkSize + 3, false},
		{base + 3, 9, false},
		{9, 9, false},
	}
	for _, c := range cases {
		if got := SameEphemeralCopy(c.a, c.b); got != c.want {
			t.Errorf("SameEphemeralCopy(%d, %d) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
