package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

const playerLoafItemId = 999962

// #277: the player's refusal keeps its words now the check is shared.
func TestEat_SpoiledFoodIsRefused(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		playerLoafItemId: {ItemId: playerLoafItemId, Name: "Stale Loaf", NameSimple: "loaf", Type: items.Food,
			Subtype: items.Edible, Uses: 1,
			Aging: items.AgingThresholds{FermentRounds: 10, PeakRounds: 20, DecayRounds: 30, SpoilRounds: 40}},
	}))
	origRound := util.GetRoundCount()
	util.SetRoundCountForTest(1000)
	t.Cleanup(func() { util.SetRoundCountForTest(origRound) })
	user, room := getTestUserAndRoom(t)
	orig := user.Character.Items
	t.Cleanup(func() { user.Character.Items = orig })
	loaf := items.New(playerLoafItemId)
	loaf.CraftedRound = 1
	user.Character.Items = []items.Item{loaf}
	events.DrainQueuedMessagesForTest(user.UserId)

	Eat("loaf", user, room, 0)

	assert.Contains(t, sentTo(user), "The food has gone bad!")
	assert.Len(t, user.Character.Items, 1, "the loaf is not eaten")
}
