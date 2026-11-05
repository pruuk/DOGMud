package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bloomDrinker quaffs item as a player set up by setup, and returns what the
// player read.
func bloomDrinker(t *testing.T, itemId int, rest string, setup func(c *characters.Character)) (*characters.Character, string) {
	t.Helper()
	seedParityWorld(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		bloomWithdrawalConditionId: {ConditionId: bloomWithdrawalConditionId, Name: "Withdrawal", TriggerCount: 200, RoundInterval: 1},
		93:                         {ConditionId: 93, Name: "Detox", TriggerCount: 50, RoundInterval: 1},
		90:                         {ConditionId: 90, Name: "Communion", TriggerCount: 30, RoundInterval: 1},
	}))
	c := newParityChar()
	setup(c)
	storeItem(t, c, items.New(itemId))
	u := &users.UserRecord{UserId: parityUserId, Character: c}
	events.DrainQueuedMessagesForTest(parityUserId)
	res := Quaff(NewUserActorInRoom(u, newEmptyTestRoom(t)).(DrinkActor), rest)
	require.True(t, res.Drank)
	return c, strings.Join(strings.Fields(strings.Join(events.DrainQueuedMessagesForTest(parityUserId), " ")), " ")
}

func withdrawing(addiction int) func(c *characters.Character) {
	return func(c *characters.Character) {
		c.BloomAddiction = addiction
		c.BloomLastDoseRound = 1234
		if err := c.AddConditionScaled(bloomWithdrawalConditionId, 1.0); err != nil {
			panic(err)
		}
	}
}

// A purge that reaches zero ends the withdrawal and says so, as the slow way does.
func TestYsoldesPurge_ReachingCleanEndsTheWithdrawal(t *testing.T) {
	c, out := bloomDrinker(t, ysoldesPurgeItemId, "ysoldes purge", withdrawing(3))
	assert.Equal(t, 0, c.BloomAddiction)
	c.Conditions.Prune()
	assert.False(t, c.HasCondition(bloomWithdrawalConditionId), "no withdrawal runs on with nothing left to crave")
	assert.Contains(t, out, "The craving has finally loosened its grip")
	assert.Zero(t, c.BloomLastDoseRound, "a clean character has no dose clock to save")
}

// Short of zero the craving is still there, and so is the withdrawal.
func TestYsoldesPurge_ShortOfCleanKeepsTheWithdrawal(t *testing.T) {
	c, out := bloomDrinker(t, ysoldesPurgeItemId, "ysoldes purge", withdrawing(8))
	assert.Equal(t, 3, c.BloomAddiction)
	c.Conditions.Prune()
	assert.True(t, c.HasCondition(bloomWithdrawalConditionId))
	assert.NotContains(t, out, "finally loosened")
}

// A purge drunk with nothing to purge cannot make anyone clean.
func TestYsoldesPurge_OnTheCleanSaysNothingOfIt(t *testing.T) {
	_, out := bloomDrinker(t, ysoldesPurgeItemId, "ysoldes purge", func(*characters.Character) {})
	assert.NotContains(t, out, "finally loosened")
}

// A wafer is not a drink: it dissolves on the tongue a few lines later.
func TestBloomWafer_IsTakenAndNotDrunk(t *testing.T) {
	_, out := bloomDrinker(t, bloomWaferItemId, "bloom wafer", func(*characters.Character) {})
	assert.Contains(t, out, "You take the")
	assert.NotContains(t, out, "You drink")
	assert.Contains(t, out, "dissolves to nothing on your tongue")

	_, out = bloomDrinker(t, ysoldesPurgeItemId, "ysoldes purge", func(*characters.Character) {})
	assert.Contains(t, out, "You drink", "every other potion is still drunk")
}
