package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// itemExcluded hides a node, pattern or greeting from a player who already
// carries the item (backpack or worn). It exists so a quest giver's handout
// ("ask iden shot", "ask delk salt") gives the item once, not once per ask,
// while the quest is open (2026-10-17 newbie-area audit, S8). The refusal is
// an authored node later in the list with the same triggers and no gate.
func TestTreeAdvance_ItemExcludedHidesAHandoutFromAHolder(t *testing.T) {
	const mobInst = 6110
	df := &DialogueFile{
		MobId: mobInst, Zone: "test", DefaultMood: "neutral",
		Tree: &Tree{Nodes: []TreeNode{
			{Id: "supply", Triggers: []string{"shot"}, GivesItem: 30064, ItemExcluded: 30064, Text: "Take a pouch."},
			{Id: "supply-held", Triggers: []string{"shot"}, Text: "You have a pouch already."},
		}},
	}
	carries := false
	var given []int
	ps := &PlayerState{
		HasQuest:    func(string) bool { return false },
		CarriesItem: func(id int) bool { return carries && id == 30064 },
		GiveItem:    func(id int) bool { given = append(given, id); return true },
	}

	text, _, _, _ := TreeAdvance(df, mobInst, 620, "shot", ps)
	assert.Equal(t, "Take a pouch.", text)
	assert.Equal(t, []int{30064}, given)

	carries = true
	ResetMemory(mobInst, 620)
	text, _, _, _ = TreeAdvance(df, mobInst, 620, "shot", ps)
	assert.Equal(t, "You have a pouch already.", text)
	assert.Equal(t, []int{30064}, given, "a player who carries the item gets no second one")

	ResetMemory(mobInst, 620)
	delete(moodCache, mobInst)
}

func TestMatch_ItemExcludedHidesAPattern(t *testing.T) {
	const mobInst = 6111
	df := &DialogueFile{
		MobId: mobInst, Zone: "test", DefaultMood: "neutral",
		Patterns: []Pattern{
			{Keywords: []string{"salt"}, ItemExcluded: 40017, GivesItem: 40017, Responses: []string{"Here is salt."}},
			{Keywords: []string{"salt"}, Responses: []string{"You have salt."}},
		},
	}
	ps := &PlayerState{
		HasQuest:    func(string) bool { return false },
		CarriesItem: func(id int) bool { return id == 40017 },
		GiveItem:    func(int) bool { return true },
	}
	text, _, _ := Match(df, mobInst, "salt", ps)
	assert.Equal(t, "You have salt.", text)
	delete(moodCache, mobInst)
}

func TestGreet_ItemExcludedHidesAVariant(t *testing.T) {
	const mobInst = 6112
	df := &DialogueFile{
		MobId: mobInst, Zone: "test", DefaultMood: "neutral",
		Tree: &Tree{Root: TreeRoot{
			Text: "Hello.",
			Variants: []QuestGreeting{
				{ItemExcluded: 10038, Text: "You have no sling."},
			},
		}},
	}
	ps := &PlayerState{
		HasQuest:    func(string) bool { return false },
		CarriesItem: func(id int) bool { return id == 10038 },
	}
	text, _, _ := Greet(df, mobInst, 630, ps)
	assert.Equal(t, "Hello.", text)
	ResetMemory(mobInst, 630)
	delete(moodCache, mobInst)
}
