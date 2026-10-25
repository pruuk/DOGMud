package conversation

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/stretchr/testify/require"
)

func toke() *dialogue.DialogueFile {
	return &dialogue.DialogueFile{MobId: 9116, Topics: []dialogue.Topic{
		{Key: "footing_in", Kind: dialogue.KindHandIn, Quest: 31, Label: "Find Your Footing", Text: "t", Effects: dialogue.Effects{GrantsQuest: "31-end"}},
		{Key: "blood", Kind: dialogue.KindOffer, Quest: 32, Label: "First Blood", Text: "t", Gates: dialogue.Gates{QuestRequired: []string{"31-end"}, QuestExcluded: []string{"32-start"}}},
		{Key: "heat", Kind: dialogue.KindOffer, Quest: 35, Label: "First Heat", Text: "t", Gates: dialogue.Gates{QuestRequired: []string{"31-end"}, QuestExcluded: []string{"35-start"}}},
		{Key: "other", Kind: dialogue.KindOffer, Quest: 60, Label: "Other", Text: "t"},
	}}
}

func held(tokens ...string) *dialogue.PlayerState {
	set := map[string]bool{}
	for _, t := range tokens {
		set[t] = true
	}
	return &dialogue.PlayerState{
		HasQuest:  func(t string) bool { return set[t] },
		HasHeard:  func(string) bool { return false },
		QuestOpen: func(int) bool { return true },
	}
}

func TestNextOffers_DerivedFromGatesOnTheFinalToken(t *testing.T) {
	got := nextOffers(toke(), "31-end", held("31-end"))
	require.Equal(t, []string{"blood", "heat"}, got, "an ungated offer is not a next offer")
}

func TestNextOffers_OneLeftIsPitched(t *testing.T) {
	got := nextOffers(toke(), "31-end", held("31-end", "32-start"))
	require.Equal(t, []string{"heat"}, got)
}

func TestNextOffers_NoneWhenNotEligible(t *testing.T) {
	require.Empty(t, nextOffers(toke(), "31-end", held()))
}

func TestTalkNextOffer_RunsAfterCompletionLines(t *testing.T) {
	require.Less(t, 0, events.TalkNextOfferPriority)
	require.Less(t, events.TalkNextOfferPriority, 50, "before a ShowLast menu (users.menuShowLastPriority)")
}

// The queue pops a TalkNextOffer after a priority-0 line queued later in the
// same pass, so the pitch follows the completion lines.
func TestTalkNextOffer_DrainsAfterALaterPriorityZeroLine(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	events.AddToQueue(events.TalkNextOffer{UserId: 1, MobInstanceId: 2, TopicKey: "k"}, events.TalkNextOfferPriority)
	events.AddToQueue(events.Message{UserId: 1, Text: "done"})
	require.Equal(t, []string{"Message", "TalkNextOffer"}, events.QueuedTypesInOrderForTest())
}
