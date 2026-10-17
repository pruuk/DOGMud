package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// A crit disarm returns a mob's weapon to its pack, so a NeverDrops weapon (the
// arena pit fighters' gear) can sit in carried Items. Death never drops it;
// a successful steal must not take it either, or a thief walks off with gear
// that exists only to be worn by that mob.
func TestTakeFromMob_NeverStealsANeverDropsItem(t *testing.T) {
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		1: {ItemId: 1, Name: "Pit Knife", Type: items.Weapon, NeverDrops: true},
		3: {ItemId: 3, Name: "Copper Ring"},
	})
	defer cleanup()

	for i := 0; i < 50; i++ {
		m := newStealTestMob(testMobInstId, 0, 1)
		m.Character.Items = []items.Item{{ItemId: 1}, {ItemId: 3}}
		thief := newStealPlayerActor(150, 5)

		res := takeFromMob(thief, m, nil)

		assert.NotEqual(t, 1, res.StoleItemId, "a NeverDrops item must never be stolen")
		assert.Equal(t, 3, res.StoleItemId, "the ordinary carried item is the only thing to take")
	}

	// Only a NeverDrops item in the pack: nothing to take.
	m := newStealTestMob(testMobInstId, 0, 1)
	m.Character.Items = []items.Item{{ItemId: 1}}
	res := takeFromMob(newStealPlayerActor(150, 5), m, nil)
	assert.Equal(t, 0, res.StoleItemId)
	assert.Len(t, m.Character.Items, 1, "the NeverDrops item stays with the mob")
}
