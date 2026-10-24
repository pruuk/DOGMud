package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// chargesGold takes the price as the first effect Apply runs, before anything
// is granted. A gold check that passes and a charge that then fails (the gold
// left between the two) must grant nothing and report false, so the caller
// says the refusal. Ported from the old TreeAdvance/Match charge tests
// (#375 Task 21).
func chargeFixture() *DialogueFile {
	return &DialogueFile{MobId: 6200, Topics: []Topic{{
		Key: "teach_razor", Kind: KindInfo, Label: "Teach me the razor.", Text: "Listen closely.",
		Gates:   Gates{GoldRequired: 25000},
		Effects: Effects{GrantsQuest: "80-start", ChargesGold: 25000},
	}}}
}

func TestApply_ChargesGold_PaysThenGrants(t *testing.T) {
	gold := 25000
	var granted []string
	var charged []int
	ps := &PlayerState{
		QuestOpen: func(int) bool { return true },
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
	df := chargeFixture()
	tp := df.FindTopic("teach_razor")
	assert.True(t, df.Eligible(tp, ps))
	assert.True(t, df.Apply(tp, ps))
	assert.Equal(t, []int{25000}, charged)
	assert.Equal(t, 0, gold)
	assert.Equal(t, []string{"80-start"}, granted)
}

func TestApply_ChargesGold_FailedChargeGrantsNothing(t *testing.T) {
	var granted []string
	ps := &PlayerState{
		QuestOpen: func(int) bool { return true },
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(tok string) { granted = append(granted, tok) },
		// The check says yes; by the time of the charge the gold is gone.
		HasGold:    func(int) bool { return true },
		ChargeGold: func(int) bool { return false },
	}
	df := chargeFixture()
	tp := df.FindTopic("teach_razor")
	assert.True(t, df.Eligible(tp, ps), "fixture check: the gate passes")
	assert.False(t, df.Apply(tp, ps), "a failed charge refuses the topic")
	assert.Empty(t, granted, "a failed charge must grant nothing")
}

func TestApply_ChargesGold_NilCallbackFailsClosed(t *testing.T) {
	var granted []string
	ps := &PlayerState{
		QuestOpen: func(int) bool { return true },
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(tok string) { granted = append(granted, tok) },
		HasGold:   func(int) bool { return true },
	}
	df := chargeFixture()
	assert.False(t, df.Apply(df.FindTopic("teach_razor"), ps))
	assert.Empty(t, granted)
}

// A paid topic whose item cannot be delivered gives the gold back and grants
// nothing.
func TestApply_ChargesGold_RefundsOnFailedDelivery(t *testing.T) {
	gold := 500
	df := &DialogueFile{MobId: 6201, Topics: []Topic{{
		Key: "buy", Kind: KindInfo, Label: "I want to buy one.", Text: "Here.",
		Gates:   Gates{GoldRequired: 500},
		Effects: Effects{ChargesGold: 500, GivesItem: 1, GrantsQuest: "x-start"},
	}}}
	var granted []string
	ps := &PlayerState{
		QuestOpen:  func(int) bool { return true },
		HasQuest:   func(string) bool { return false },
		GiveQuest:  func(tok string) { granted = append(granted, tok) },
		HasGold:    func(a int) bool { return gold >= a },
		ChargeGold: func(a int) bool { gold -= a; return true },
		GiveGold:   func(a int) { gold += a },
		GiveItem:   func(int) bool { return false },
	}
	assert.False(t, df.Apply(df.FindTopic("buy"), ps))
	assert.Equal(t, 500, gold, "the charge is refunded when the item cannot be delivered")
	assert.Empty(t, granted)
}
