package items

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// TestShouldDrop_ZeroDefaultMeansNever pins #585 at the primitive: a caller's
// defaultChance of 0 means never (the caller has already resolved an absent
// itemdropchance to its own default), 100 means always, a positive chance in
// between rolls as before, and an item's own positive DropChance overrides the
// caller's chance either way. Before the fix a defaultChance of 0 became 100.
func TestShouldDrop_ZeroDefaultMeansNever(t *testing.T) {
	defer util.SetRandForTest(585)()
	const rolls = 1000
	cases := []struct {
		name      string
		own, dflt int
		lo, hi    int // inclusive bounds on drops out of rolls
	}{
		{"default 0 is never", 0, 0, 0, 0},
		{"default 100 is always", 0, 100, rolls, rolls},
		{"default 50 rolls about half", 0, 50, 400, 600},
		{"own 100 overrides default 0", 100, 0, rolls, rolls},
		{"own 30 overrides default 0", 30, 0, 200, 400},
		{"own 30 overrides default 100", 30, 100, 200, 400},
	}
	for _, c := range cases {
		it := Item{ItemId: 1, DropChance: c.own}
		drops := 0
		for i := 0; i < rolls; i++ {
			if it.ShouldDrop(c.dflt) {
				drops++
			}
		}
		if drops < c.lo || drops > c.hi {
			t.Errorf("%s: %d drops of %d, want %d..%d", c.name, drops, rolls, c.lo, c.hi)
		}
	}
}
