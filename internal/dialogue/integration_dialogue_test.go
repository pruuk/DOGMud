package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─── Integration: topics with quest gates ────────────────────────────────────
// Exercises: Eligible, Apply, Menu, checkQuestGate, applyQuestEffects,
// PlayerState. Ported from the old Match/TreeAdvance/Greet integration tests
// (#375 Task 21).

func TestIntegration_OfferThenFollowUpWithQuestGates(t *testing.T) {
	df := &DialogueFile{
		MobId: 1,
		Opener: &Opener{
			Text:     "Hello there.",
			Variants: []OpenerVariant{{Gates: Gates{QuestRequired: []string{"9-saved"}}, Text: "Thank you for saving us!"}},
		},
		Topics: []Topic{
			{Key: "herbs", Kind: KindOffer, Quest: 9, Label: "The Herb Run", PlayerLine: "Do you need help?",
				Text:    "Bring me 5 herbs.",
				Gates:   Gates{QuestExcluded: []string{"9-start", "9-end"}},
				Effects: Effects{GrantsQuest: "9-start"}},
			{Key: "herbs_again", Kind: KindInfo, Label: "About those herbs.",
				Text:  "You already accepted my task!",
				Gates: Gates{QuestRequired: []string{"9-start"}}},
			{Key: "hello", Kind: KindInfo, Label: "Hello.", Text: "Greetings, traveler."},
		},
	}

	tokens := map[string]bool{}
	ps := &PlayerState{
		QuestOpen:  func(int) bool { return true },
		HasQuest:   func(token string) bool { return tokens[token] },
		HasItem:    func(itemId int) bool { return false },
		RemoveItem: func(itemId int) bool { return false },
		GiveQuest:  func(token string) { tokens[token] = true },
	}
	ids := func() []string {
		var out []string
		for _, e := range df.Menu("Elder", ps).Entries {
			out = append(out, e.Id)
		}
		return out
	}

	// Before the quest: the offer is on the menu, the follow-up is not.
	assert.Equal(t, []string{"herbs", "hello"}, ids())
	text, _ := df.OpenerFor(ps)
	assert.Equal(t, "Hello there.", text)

	// Choosing the offer grants the quest.
	assert.True(t, df.Apply(df.FindTopic("herbs"), ps))
	assert.True(t, tokens["9-start"], "the quest is granted when the offer is chosen")

	// Holding the quest: the offer is gone and the follow-up has appeared.
	assert.Equal(t, []string{"herbs_again", "hello"}, ids())

	// The opener variant answers once its token is held.
	tokens["9-saved"] = true
	text, _ = df.OpenerFor(ps)
	assert.Equal(t, "Thank you for saving us!", text)

	// A nil PlayerState skips the quest checks (backward compat).
	assert.True(t, df.Eligible(df.FindTopic("herbs"), nil))
	assert.True(t, df.Eligible(df.FindTopic("herbs_again"), nil))
}

// heardRequired replaces the old tree's requires/unlocks: a chain of info
// topics opens one step at a time as each is heard, and the record is the
// saved heard set, not per-instance memory.
func TestIntegration_HeardRequiredChain(t *testing.T) {
	df := &DialogueFile{MobId: 3, Topics: []Topic{
		{Key: "village", Kind: KindInfo, Label: "Tell me about the village.", Text: "The village is peaceful."},
		{Key: "elder", Kind: KindInfo, Label: "Who is the elder?", Text: "The elder knows many secrets.", HeardRequired: []string{"village"}},
		{Key: "forest", Kind: KindInfo, Label: "What of the forest?", Text: "The forest is dark and dangerous.", HeardRequired: []string{"elder"}},
	}}
	heard := map[string]bool{}
	ps := &PlayerState{
		QuestOpen: func(int) bool { return true },
		HasHeard:  func(k string) bool { return heard[k] },
		MarkHeard: func(k string) { heard[k] = true },
	}
	open := func(key string) bool { return df.Eligible(df.FindTopic(key), ps) }

	assert.True(t, open("village"))
	assert.False(t, open("elder"), "elder needs village heard")
	assert.False(t, open("forest"), "forest needs elder heard")

	assert.True(t, df.Apply(df.FindTopic("village"), ps))
	assert.True(t, heard[HeardKey(3, "village")])
	assert.True(t, open("elder"))
	assert.False(t, open("forest"))

	assert.True(t, df.Apply(df.FindTopic("elder"), ps))
	assert.True(t, open("forest"))
}

func TestIntegration_MoodManagement(t *testing.T) {
	mobId := 99

	// Default mood
	mood := GetMood(mobId, "neutral")
	assert.Equal(t, MoodNeutral, mood)

	// Set mood directly
	SetMood(mobId, MoodFriendly)
	mood = GetMood(mobId, "neutral")
	assert.Equal(t, MoodFriendly, mood)

	// Shift mood
	ShiftMood(mobId, "hostile", "neutral")
	mood = GetMood(mobId, "neutral")
	assert.Equal(t, MoodHostile, mood)

	// Shift with empty target should not change
	ShiftMood(mobId, "", "neutral")
	mood = GetMood(mobId, "neutral")
	assert.Equal(t, MoodHostile, mood, "Empty shift should not change mood")

	// Clean up
	delete(moodCache, mobId)
}
