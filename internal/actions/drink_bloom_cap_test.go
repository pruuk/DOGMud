package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drinkWaferAtCap drinks a Bloom Wafer as a player who holds owned, with
// MutationMaxCount 2 and the given roll chances, and returns the character.
func drinkWaferAtCap(t *testing.T, owned map[string]int, newChance, advanceChance float64) (DrinkResult, *characters.Character, []parityQueued) {
	t.Helper()
	seedParityWorld(t)
	cfg := configs.GetConfig()
	cfg.Balance.MutationMaxCount = 2
	cfg.Balance.BloomNewMutationChance = configs.ConfigFloat(newChance)
	cfg.Balance.BloomMutationAdvanceChance = configs.ConfigFloat(advanceChance)
	configs.SetConfigForTest(t, cfg)

	room := newEmptyTestRoom(t)
	c := newParityChar()
	c.Mutations = owned
	storeItem(t, c, items.New(bloomWaferItemId))
	u := &users.UserRecord{UserId: parityUserId, Character: c}
	events.DrainQueuedConditionsForTest(parityUserId)
	res := Quaff(NewUserActorInRoom(u, room).(DrinkActor), "bloom wafer")
	return res, c, parityQueuedOf(events.DrainQueuedConditionsForTest(parityUserId))
}

// A player at MutationMaxCount whose mutations are all at their rank cap still
// gets the wafer: it is consumed, the high and the addiction land, and no
// mutation changes. With nothing granted, drink.go skips the "shifts and
// settles" line, so the player is never told something changed that did not.
func TestDrinkBloomWafer_CappedPlayerKeepsTheHighGainsNothing(t *testing.T) {
	top := mutations.MaxRankOf("common-1")
	for name, rolls := range map[string][2]float64{"new mutation roll": {1.0, 0}, "advance roll": {0, 1.0}} {
		t.Run(name, func(t *testing.T) {
			res, c, queued := drinkWaferAtCap(t, map[string]int{"common-1": top, "rare-1": top}, rolls[0], rolls[1])
			require.True(t, res.Drank, "the wafer is drunk")
			assert.Equal(t, map[string]int{"common-1": top, "rare-1": top}, c.Mutations, "no mutation changes")
			assert.NotContains(t, c.Mutations, "rare-2")
			assert.Equal(t, 0, countDrinkItemsById(c.Items, bloomWaferItemId), "the wafer is consumed")
			assert.Equal(t, 1, c.BloomAddiction, "the dose still counts")
			_, ok := queuedHas(queued, 90)
			assert.True(t, ok, "Communion still lands")
		})
	}
}

// Control: the same wafer at the count still deepens an owned mutation, so the
// tests above fail for the cap and not because the roll never reaches a grant.
func TestDrinkBloomWafer_AtTheCountStillDeepens(t *testing.T) {
	top := mutations.MaxRankOf("common-1")
	if top < 2 {
		t.Skip("needs a mutation rank cap of at least 2")
	}
	res, c, _ := drinkWaferAtCap(t, map[string]int{"common-1": 1, "rare-1": top}, 0, 1.0)
	require.True(t, res.Drank)
	assert.Equal(t, 2, c.Mutations["common-1"], "the owned mutation deepens")
	assert.Len(t, c.Mutations, 2)
}
