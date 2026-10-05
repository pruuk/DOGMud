package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// chargesGold takes the price in the same step that matches the node, before
// anything is granted. A gold check that passes and a charge that then fails
// (the gold left between the two) must grant nothing and fall through to the
// next matching node, the authored refusal.
func chargeFixture() *DialogueFile {
	return &DialogueFile{
		MobId:       6200,
		DefaultMood: "neutral",
		Tree: &Tree{Nodes: []TreeNode{
			{
				Id:           "teach",
				Triggers:     []string{"teach razor"},
				GrantsQuest:  "80-start",
				GoldRequired: 25000,
				ChargesGold:  25000,
				Text:         "Listen closely.",
			},
			{Id: "refuse", Triggers: []string{"teach razor"}, Text: "You can't pay."},
		}},
	}
}

func TestTreeAdvance_ChargesGold_PaysThenGrants(t *testing.T) {
	gold := 25000
	var granted []string
	var charged []int
	ps := &PlayerState{
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(tok string) { granted = append(granted, tok) },
		HasGold:   func(a int) bool { return gold >= a },
		ChargeGold: func(a int) bool {
			if gold < a {
				return false
			}
			gold -= a
			charged = append(charged, a)
			return true
		},
	}
	text, _, _, _ := TreeAdvance(chargeFixture(), 6200, 630, "teach razor", ps)
	assert.Equal(t, "Listen closely.", text)
	assert.Equal(t, []int{25000}, charged)
	assert.Equal(t, 0, gold)
	assert.Equal(t, []string{"80-start"}, granted)
	ResetMemory(6200, 630)
}

func TestTreeAdvance_ChargesGold_FailedChargeGrantsNothing(t *testing.T) {
	var granted []string
	ps := &PlayerState{
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(tok string) { granted = append(granted, tok) },
		// The check says yes; by the time of the charge the gold is gone.
		HasGold:    func(int) bool { return true },
		ChargeGold: func(int) bool { return false },
	}
	text, _, _, _ := TreeAdvance(chargeFixture(), 6200, 631, "teach razor", ps)
	assert.Equal(t, "You can't pay.", text, "a failed charge falls through to the refusal")
	assert.Empty(t, granted, "a failed charge must grant nothing")
	ResetMemory(6200, 631)
}

func TestTreeAdvance_ChargesGold_NilCallbackFailsClosed(t *testing.T) {
	var granted []string
	ps := &PlayerState{
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(tok string) { granted = append(granted, tok) },
		HasGold:   func(int) bool { return true },
	}
	text, _, _, _ := TreeAdvance(chargeFixture(), 6200, 632, "teach razor", ps)
	assert.Equal(t, "You can't pay.", text)
	assert.Empty(t, granted)
	ResetMemory(6200, 632)
}

// A paid node whose item cannot be delivered gives the gold back.
func TestTreeAdvance_ChargesGold_RefundsOnFailedDelivery(t *testing.T) {
	gold := 500
	df := &DialogueFile{MobId: 6201, DefaultMood: "neutral", Tree: &Tree{Nodes: []TreeNode{
		{Id: "buy", Triggers: []string{"buy"}, ChargesGold: 500, GoldRequired: 500, GivesItem: 1, GrantsQuest: "x-start", Text: "Here."},
	}}}
	var granted []string
	ps := &PlayerState{
		HasQuest:   func(string) bool { return false },
		GiveQuest:  func(tok string) { granted = append(granted, tok) },
		HasGold:    func(a int) bool { return gold >= a },
		ChargeGold: func(a int) bool { gold -= a; return true },
		GiveGold:   func(a int) { gold += a },
		GiveItem:   func(int) bool { return false },
	}
	TreeAdvance(df, 6201, 633, "buy", ps)
	assert.Equal(t, 500, gold, "the charge is refunded when the item cannot be delivered")
	assert.Empty(t, granted)
	ResetMemory(6201, 633)
}

func TestMatch_ChargesGold(t *testing.T) {
	gold := 100
	df := &DialogueFile{MobId: 6202, DefaultMood: "neutral", Patterns: []Pattern{
		{Keywords: []string{"teach"}, ChargesGold: 100, GoldRequired: 100, GrantsQuest: "y-start", Responses: []string{"paid"}},
		{Keywords: []string{"teach"}, Responses: []string{"short"}},
	}}
	var granted []string
	ps := &PlayerState{
		HasQuest:   func(string) bool { return false },
		GiveQuest:  func(tok string) { granted = append(granted, tok) },
		HasGold:    func(int) bool { return true },
		ChargeGold: func(a int) bool { return false },
	}
	resp, _, _ := Match(df, 6202, "teach", ps)
	assert.Equal(t, "short", resp)
	assert.Empty(t, granted)

	ps.ChargeGold = func(a int) bool { gold -= a; return true }
	resp, _, _ = Match(df, 6202, "teach", ps)
	assert.Equal(t, "paid", resp)
	assert.Equal(t, 0, gold)
	assert.Equal(t, []string{"y-start"}, granted)
	delete(moodCache, 6202)
}
