package quests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testNames() RewardNames {
	items := map[int]string{30036: "Healing Salve", 9001: "Iron Lockpicks", 9002: "Oak Bow"}
	return RewardNames{
		Item: func(id int) string { return items[id] },
		Recipe: func(id string) string {
			return map[string]string{"iron-dagger": "Iron Dagger"}[id]
		},
		Spell:   func(id string) string { return map[string]string{"glow": "Glow"}[id] },
		Faction: func(id string) string { return map[string]string{"caravan": "the Caravan Guild"}[id] },
	}
}

func TestRewardSummary_Table(t *testing.T) {
	cases := []struct {
		name string
		r    QuestReward
		want string
	}{
		{"none", QuestReward{}, ""},
		{"gold", QuestReward{Gold: 6}, "6 gold"},
		{"gold and skill", QuestReward{Gold: 6, SkillInfo: "weapon-combat:1"}, "6 gold and training in weapon combat"},
		{"item article", QuestReward{ItemId: 9002}, "an Oak Bow"},
		{"plural item", QuestReward{ItemId: 9001}, "some Iron Lockpicks"},
		{"stockpile counts", QuestReward{ItemInfo: "30036:3,9002"}, "Healing Salve (x3) and an Oak Bow"},
		{"recipe", QuestReward{RecipeInfo: "iron-dagger"}, "the recipe for Iron Dagger"},
		{"spell", QuestReward{SpellId: "glow"}, "the spell Glow"},
		{"stat", QuestReward{StatInfo: "strength:1"}, "more Strength"},
		{"rep", QuestReward{RepFaction: "caravan", RepAmount: 5}, "the goodwill of the Caravan Guild"},
		{"rep loss hidden", QuestReward{RepFaction: "caravan", RepAmount: -5}, ""},
		{"three joins", QuestReward{Gold: 2, ItemId: 9002, SpellId: "glow"}, "2 gold, an Oak Bow, and the spell Glow"},
		{"silent kinds", QuestReward{ConditionId: 3, RoomId: 10, QuestId: "5-start"}, ""},
		{"unknown name skipped", QuestReward{ItemId: 4242, Gold: 1}, "1 gold"},
	}
	for _, c := range cases {
		q := &Quest{Rewards: c.r}
		require.Equal(t, c.want, RewardSummary(q, testNames()), c.name)
	}
}

func TestRewardSummary_OverrideWins(t *testing.T) {
	q := &Quest{Rewards: QuestReward{Gold: 6, Summary: "Garve will make it worth your while."}}
	require.Equal(t, "Garve will make it worth your while.", RewardSummary(q, testNames()))
}

func TestRewardPaidNames(t *testing.T) {
	q := &Quest{Rewards: QuestReward{ItemId: 9002, ItemInfo: "30036:2", RecipeInfo: "iron-dagger", SpellId: "glow", Gold: 4}}
	require.Equal(t, []string{"Oak Bow", "Healing Salve", "Iron Dagger", "Glow"}, RewardPaidNames(q, testNames()))
}
