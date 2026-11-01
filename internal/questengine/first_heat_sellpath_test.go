package questengine

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// R22 (#375) let a sale of the crafted iron dagger to Smith Rusk complete
// First Heat (35). #375 phase 7 ends every quest at a hand-in (ruleEnd), so
// the sale now only draws Rusk's line, and the player tells her how the work
// went (craft_turnin_told, pinned in newbie_topic_steps_test.go). These load
// the REAL quest 35 file and fire the command event the sell action raises.

const (
	sellPathRusk   = 9116
	sellPathDagger = 10009
	sellPathForge  = 5245
)

func loadQuest35ForSell(t *testing.T) (*Engine, *QuestDef) {
	t.Helper()
	data, err := os.ReadFile("../../_datafiles/world/dogmud/quests/35-first_heat.yaml")
	require.NoError(t, err, "must be able to read the real quest 35 YAML")
	var q QuestDef
	require.NoError(t, yaml.Unmarshal(data, &q))
	require.Equal(t, 35, q.QuestId)
	e := NewEngine()
	e.RegisterQuest(&q)
	return e, &q
}

func crafted35Player() *fullMockPlayer {
	p := newFullMockPlayer(sellPathForge)
	p.quests["35-start"] = true
	p.quests["35-craft"] = true
	return p
}

func TestQuest35_NoTriggerGrantsTheEnd(t *testing.T) {
	_, q := loadQuest35ForSell(t)
	for i, tr := range q.Triggers {
		for _, a := range tr.Actions {
			require.NotEqual(t, "35-end", a.Grant, "trigger %d grants 35-end; only Rusk's hand-in may", i)
		}
	}
}

func TestQuest35_SellingDaggerToRuskPointsToTheHandIn(t *testing.T) {
	e, _ := loadQuest35ForSell(t)
	player := crafted35Player()
	ctx := newFullMockActionContext(1, player)

	result := e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: sellPathDagger, MobId: sellPathRusk,
	}, player, ctx)

	assert.True(t, result.Handled, "selling the dagger to Rusk draws her line")
	assert.False(t, player.HasQuest("35-end"), "the sale alone does not complete First Heat")
	require.Len(t, ctx.npcSays, 1, "Rusk must answer the sale")
	assert.Equal(t, sellPathRusk, ctx.npcSays[0].Mob)
	require.Len(t, ctx.npcSays[0].Lines, 1)
	assert.Equal(t, "I love the entrepreneurial spirit. Now tell me how the work went.",
		ctx.npcSays[0].Lines[0].Text)
}

func TestQuest35_SellingOtherItemToRuskSaysNothing(t *testing.T) {
	e, _ := loadQuest35ForSell(t)
	player := crafted35Player()
	ctx := newFullMockActionContext(1, player)

	e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: 40001, MobId: sellPathRusk,
	}, player, ctx)

	assert.Empty(t, ctx.npcSays, "selling an iron ingot is not the dagger")
}

func TestQuest35_SellingDaggerToAnotherShopSaysNothing(t *testing.T) {
	e, _ := loadQuest35ForSell(t)
	player := crafted35Player()
	ctx := newFullMockActionContext(1, player)

	e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: sellPathDagger, MobId: 9117,
	}, player, ctx)

	assert.Empty(t, ctx.npcSays, "only Rusk's shop answers")
}

func TestQuest35_SellingDaggerBeforeCraftingSaysNothing(t *testing.T) {
	e, _ := loadQuest35ForSell(t)
	player := newFullMockPlayer(sellPathForge)
	player.quests["35-start"] = true
	ctx := newFullMockActionContext(1, player)

	e.Notify("command", EventDetails{
		UserId: 1, RoomId: sellPathForge, Command: "sell",
		ItemId: sellPathDagger, MobId: sellPathRusk,
	}, player, ctx)

	assert.Empty(t, ctx.npcSays, "the line waits for 35-craft")
	assert.False(t, player.HasQuest("35-end"))
}
