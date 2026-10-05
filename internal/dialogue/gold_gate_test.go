package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTreeAdvance_GoldGate exercises the goldRequired gate on a TreeNode.
// A node that grants a priced quest must not match for a player who cannot
// pay: the quest's own charge_gold runs on quest_granted, so the dialogue is
// the only place that can refuse before anything is granted. The refusal is
// an authored node later in the list with the same triggers and no gate, the
// same shape masterworkRequired uses.
func TestTreeAdvance_GoldGate(t *testing.T) {
	const mobInst = 6100
	df := &DialogueFile{
		MobId:       mobInst,
		Zone:        "test",
		DefaultMood: "neutral",
		Tree: &Tree{
			Nodes: []TreeNode{
				{
					Id:           "offer",
					Triggers:     []string{"blackrazor"},
					GrantsQuest:  "80-start",
					GoldRequired: 25000,
					Text:         "Twenty-five thousand. It's yours.",
				},
				{
					Id:       "offer-refused",
					Triggers: []string{"blackrazor"},
					Text:     "Twenty-five thousand. You don't have it.",
				},
			},
		},
	}

	gold := 1400
	var granted []string
	ps := &PlayerState{
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(token string) { granted = append(granted, token) },
		HasGold:   func(amount int) bool { return gold >= amount },
	}

	// Short of the price: the priced node is hidden, nothing is granted, the
	// refusal node answers, and the player's gold is untouched.
	text, _, _, advanced := TreeAdvance(df, mobInst, 610, "blackrazor", ps)
	assert.True(t, advanced)
	assert.Equal(t, "Twenty-five thousand. You don't have it.", text)
	assert.Empty(t, granted, "a player who cannot pay must not be granted the quest")
	assert.Equal(t, 1400, gold, "the gate takes nothing")

	// Exactly the price: the priced node matches and grants.
	gold = 25000
	ResetMemory(mobInst, 610)
	text, _, _, advanced = TreeAdvance(df, mobInst, 610, "blackrazor", ps)
	assert.True(t, advanced)
	assert.Equal(t, "Twenty-five thousand. It's yours.", text)
	assert.Equal(t, []string{"80-start"}, granted)

	// A PlayerState with no HasGold callback cannot confirm the player can
	// pay, so the gate fails closed (as requiresItem does), never open.
	granted = nil
	ResetMemory(mobInst, 611)
	legacy := &PlayerState{
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(token string) { granted = append(granted, token) },
	}
	text, _, _, _ = TreeAdvance(df, mobInst, 611, "blackrazor", legacy)
	assert.Equal(t, "Twenty-five thousand. You don't have it.", text)
	assert.Empty(t, granted, "nil HasGold must not open a priced node")

	ResetMemory(mobInst, 610)
	ResetMemory(mobInst, 611)
	delete(moodCache, mobInst)
}

// TestMatch_GoldGate covers the same gate on a keyword Pattern.
func TestMatch_GoldGate(t *testing.T) {
	const mobInst = 6101
	df := &DialogueFile{
		MobId:       mobInst,
		DefaultMood: "neutral",
		Patterns: []Pattern{
			{Keywords: []string{"razor"}, GoldRequired: 500, GrantsQuest: "x-start", Responses: []string{"paid"}},
			{Keywords: []string{"razor"}, Responses: []string{"short"}},
		},
	}
	gold := 499
	var granted []string
	ps := &PlayerState{
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(token string) { granted = append(granted, token) },
		HasGold:   func(amount int) bool { return gold >= amount },
	}

	resp, _, ok := Match(df, mobInst, "razor", ps)
	assert.True(t, ok)
	assert.Equal(t, "short", resp)
	assert.Empty(t, granted)

	gold = 500
	resp, _, _ = Match(df, mobInst, "razor", ps)
	assert.Equal(t, "paid", resp)
	assert.Equal(t, []string{"x-start"}, granted)
	delete(moodCache, mobInst)
}

// TestGreet_GoldGate covers the same gate on a root greeting variant.
func TestGreet_GoldGate(t *testing.T) {
	const mobInst = 6102
	df := &DialogueFile{
		MobId:       mobInst,
		DefaultMood: "neutral",
		Tree: &Tree{Root: TreeRoot{
			Text:     "plain",
			Variants: []QuestGreeting{{GoldRequired: 100, Text: "rich"}},
		}},
	}
	gold := 50
	ps := &PlayerState{HasGold: func(amount int) bool { return gold >= amount }}

	text, _, _ := Greet(df, mobInst, 620, ps)
	assert.Equal(t, "plain", text)

	gold = 100
	text, _, _ = Greet(df, mobInst, 620, ps)
	assert.Equal(t, "rich", text)

	ResetMemory(mobInst, 620)
	delete(moodCache, mobInst)
}
