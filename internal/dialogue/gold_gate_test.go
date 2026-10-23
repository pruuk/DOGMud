package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEligible_GoldGate exercises the goldRequired gate on a topic. A topic
// that grants a priced quest must not open for a player who cannot pay: the
// quest's own charge_gold runs on quest_granted, so the dialogue is the only
// place that can refuse before anything is granted. Ported from the old
// TreeAdvance/Match gold-gate tests (#375 Task 21).
func TestEligible_GoldGate(t *testing.T) {
	df := &DialogueFile{MobId: 6100, Topics: []Topic{{
		Key: "blackrazor", Kind: KindInfo, Label: "Blackrazor", Text: "Twenty-five thousand. It's yours.",
		Gates:   Gates{GoldRequired: 25000},
		Effects: Effects{GrantsQuest: "80-start"},
	}}}
	tp := df.FindTopic("blackrazor")

	gold := 1400
	var granted []string
	ps := &PlayerState{
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(token string) { granted = append(granted, token) },
		HasGold:   func(amount int) bool { return gold >= amount },
	}

	// Short of the price: the topic is hidden and the gate takes nothing.
	assert.False(t, df.Eligible(tp, ps), "a player who cannot pay must not see the priced topic")
	assert.Empty(t, df.Menu("Veyra", ps).Entries)
	assert.Equal(t, 1400, gold, "the gate takes nothing")

	// Exactly the price: the topic opens and grants.
	gold = 25000
	assert.True(t, df.Eligible(tp, ps))
	assert.True(t, df.Apply(tp, ps))
	assert.Equal(t, []string{"80-start"}, granted)

	// A PlayerState with no HasGold callback cannot confirm the player can
	// pay, so the gate fails closed (as requiresItem does), never open.
	legacy := &PlayerState{
		HasQuest:  func(string) bool { return false },
		GiveQuest: func(string) {},
	}
	assert.False(t, df.Eligible(tp, legacy), "nil HasGold must not open a priced topic")
}

// TestOpenerFor_GoldGate covers the same gate on an opener variant.
func TestOpenerFor_GoldGate(t *testing.T) {
	df := &DialogueFile{MobId: 6102, Opener: &Opener{
		Text:     "plain",
		Variants: []OpenerVariant{{Gates: Gates{GoldRequired: 100}, Text: "rich"}},
	}}
	gold := 50
	ps := &PlayerState{HasGold: func(amount int) bool { return gold >= amount }}

	text, _ := df.OpenerFor(ps)
	assert.Equal(t, "plain", text)

	gold = 100
	text, _ = df.OpenerFor(ps)
	assert.Equal(t, "rich", text)
}
