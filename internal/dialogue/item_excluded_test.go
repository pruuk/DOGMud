package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// itemExcluded hides a topic or an opener variant from a player who already
// carries the item (backpack or worn). It exists so a quest giver's handout
// gives the item once, not once per visit, while the quest is open
// (2026-10-17 newbie-area audit, S8). Ported from the old TreeAdvance, Match
// and Greet tests (#375 Task 21).
func TestEligible_ItemExcludedHidesAHandoutFromAHolder(t *testing.T) {
	df := &DialogueFile{MobId: 6110, Topics: []Topic{{
		Key: "shot", Kind: KindInfo, Label: "Can I have some shot?", Text: "Take a pouch.",
		Gates:   Gates{ItemExcluded: 30064},
		Effects: Effects{GivesItem: 30064},
	}}}
	tp := df.FindTopic("shot")
	carries := false
	var given []int
	ps := &PlayerState{
		QuestOpen:   func(int) bool { return true },
		HasQuest:    func(string) bool { return false },
		CarriesItem: func(id int) bool { return carries && id == 30064 },
		GiveItem:    func(id int) bool { given = append(given, id); return true },
	}

	assert.True(t, df.Eligible(tp, ps))
	assert.True(t, df.Apply(tp, ps))
	assert.Equal(t, []int{30064}, given)

	carries = true
	assert.False(t, df.Eligible(tp, ps), "a player who carries the item is not offered a second one")
	assert.Empty(t, df.Menu("Iden", ps).Entries)
}

// Without CarriesItem the gate falls back to HasItem (the backpack).
func TestEligible_ItemExcludedFallsBackToHasItem(t *testing.T) {
	df := &DialogueFile{MobId: 6111, Topics: []Topic{{
		Key: "salt", Kind: KindInfo, Label: "Any salt?", Text: "Here is salt.",
		Gates: Gates{ItemExcluded: 40017},
	}}}
	ps := &PlayerState{
		QuestOpen: func(int) bool { return true },
		HasQuest:  func(string) bool { return false },
		HasItem:   func(id int) bool { return id == 40017 },
	}
	assert.False(t, df.Eligible(df.FindTopic("salt"), ps))
}

func TestOpenerFor_ItemExcludedHidesAVariant(t *testing.T) {
	df := &DialogueFile{MobId: 6112, Opener: &Opener{
		Text:     "Hello.",
		Variants: []OpenerVariant{{Gates: Gates{ItemExcluded: 10038}, Text: "You have no sling."}},
	}}
	ps := &PlayerState{
		QuestOpen:   func(int) bool { return true },
		HasQuest:    func(string) bool { return false },
		CarriesItem: func(id int) bool { return id == 10038 },
	}
	text, _ := df.OpenerFor(ps)
	assert.Equal(t, "Hello.", text)
}
