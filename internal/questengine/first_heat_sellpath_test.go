package questengine

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// R22 (#375): selling the crafted iron dagger to Smith Rusk is a turn-in of
// quest 35 "First Heat", the same as giving it to her or asking her. These
// tests load the REAL quest 35 file and fire the command event the sell action
// raises (command "sell", the item sold, the buying shop mob, the room).

const (
	sellPathRusk   = 9116
	sellPathDagger = 10009
	sellPathForge  = 5245
)

func loadQuest35ForSell(t *testing.T) *Engine {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/quests/35-first_heat.yaml")
	require.NoError(t, err, "must be able to read the real quest 35 YAML")
	var q QuestDef
	require.NoError(t, yaml.Unmarshal(data, &q))
	require.Equal(t, 35, q.QuestId)
	e := NewEngine()
	e.RegisterQuest(&q)
	return e
}

func crafted35Player() *fullMockPlayer {
	p := newFullMockPlayer(sellPathForge)
	p.quests["35-start"] = true
	p.quests["35-craft"] = true
	return p
}

func TestQuest35_SellingDaggerToRuskCompletesFirstHeat(t *testing.T) {
	e := loadQuest35ForSell(t)
	player := crafted35Player()
	ctx := newFullMockActionContext(1, player)

	result := e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: sellPathDagger, MobId: sellPathRusk,
	}, player, ctx)

	assert.True(t, result.Handled, "selling the dagger to Rusk must hit a quest trigger")
	assert.True(t, player.HasQuest("35-end"), "the sale must complete First Heat")
	require.Len(t, ctx.npcSays, 1, "Rusk must answer the sale")
	assert.Equal(t, sellPathRusk, ctx.npcSays[0].Mob)
	require.Len(t, ctx.npcSays[0].Lines, 1)
	assert.Equal(t, "I love the entrepreneurial spirit. You'll be a smith in no time.",
		ctx.npcSays[0].Lines[0].Text)
}

func TestQuest35_SellingOtherItemToRuskDoesNotComplete(t *testing.T) {
	e := loadQuest35ForSell(t)
	player := crafted35Player()
	ctx := newFullMockActionContext(1, player)

	e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: 40001, MobId: sellPathRusk,
	}, player, ctx)

	assert.False(t, player.HasQuest("35-end"), "selling an iron ingot is not the turn-in")
	assert.Empty(t, ctx.npcSays)
}

func TestQuest35_SellingDaggerToAnotherShopDoesNotComplete(t *testing.T) {
	e := loadQuest35ForSell(t)
	player := crafted35Player()
	ctx := newFullMockActionContext(1, player)

	e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: sellPathDagger, MobId: 9117,
	}, player, ctx)

	assert.False(t, player.HasQuest("35-end"), "only Rusk's shop counts")
	assert.Empty(t, ctx.npcSays)
}

func TestQuest35_SellingDaggerBeforeCraftingDoesNotComplete(t *testing.T) {
	e := loadQuest35ForSell(t)
	player := newFullMockPlayer(sellPathForge)
	player.quests["35-start"] = true
	ctx := newFullMockActionContext(1, player)

	e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: sellPathDagger, MobId: sellPathRusk,
	}, player, ctx)

	assert.False(t, player.HasQuest("35-end"), "the turn-in needs 35-craft first")
}
