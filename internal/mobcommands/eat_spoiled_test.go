package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mobLoafItemId = 999961

// seedMobLoaf gives mob 100 one loaf made at craftedRound, at round 1000.
func seedMobLoaf(t *testing.T, craftedRound uint64) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		mobLoafItemId: {ItemId: mobLoafItemId, Name: "Stale Loaf", NameSimple: "loaf", Type: items.Food,
			Subtype: items.Edible, Uses: 1,
			Aging: items.AgingThresholds{FermentRounds: 10, PeakRounds: 20, DecayRounds: 30, SpoilRounds: 40}},
	}))
	origRound := util.GetRoundCount()
	util.SetRoundCountForTest(1000)
	t.Cleanup(func() { util.SetRoundCountForTest(origRound) })
	mob, room := getTestMobAndRoom(t)
	orig := mob.Character.Items
	t.Cleanup(func() { mob.Character.Items = orig })
	loaf := items.New(mobLoafItemId)
	loaf.CraftedRound = craftedRound
	mob.Character.Items = []items.Item{loaf}
	return mob, room
}

// #277: a mob left spoiled food alone instead of eating it.
func TestMobEat_SpoiledFoodIsLeft(t *testing.T) {
	mob, room := seedMobLoaf(t, 1)
	handled, err := Eat("loaf", mob, room)
	require.True(t, handled)
	require.NoError(t, err)
	assert.Len(t, mob.Character.Items, 1, "a mob does not eat food that has gone bad")
}

func TestMobEat_FreshFoodIsEaten(t *testing.T) {
	mob, room := seedMobLoaf(t, 995)
	Eat("loaf", mob, room)
	assert.Empty(t, mob.Character.Items, "fresh food is eaten")
}
