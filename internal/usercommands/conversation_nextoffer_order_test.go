package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A hand-in that finishes quest 9 beside the one offer gated on its final
// token, plus an info topic so a re-shown talk menu would have something to
// list (and the "no talk menu" assertion below can fail).
const nextOfferFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: purse
    kind: handin
    quest: 9
    label: "The Lost Purse"
    playerLine: "Here is the purse you were looking for."
    text: "My thanks."
    requiresItem: 10001
    questRequired: [9-start]
    questExcluded: [9-end]
    grantsQuest: 9-end
  - key: ledger
    kind: offer
    quest: 12
    label: "The Missing Ledger"
    playerLine: "Is there more I can do?"
    text: "My ledger went missing too."
    questRequired: [9-end]
    questExcluded: [12-start, 12-end]
    grantsQuest: 12-start
  - key: bones
    kind: info
    label: "Why do your bones rattle?"
    text: "Old age."
`

// deliveredWithNextOffer is deliveredInOrder with the two production pieces
// the next offer needs: the quest stand-in also lands the token (as
// HandleQuestUpdate does before it prints, so the offer gated on it is
// eligible when its pitch runs), and the real TalkNextOffer listener.
// HandleQuestUpdate itself cannot run here: package hooks imports
// usercommands, so this package's tests cannot import hooks.
func deliveredWithNextOffer(t *testing.T) []string {
	t.Helper()
	restore := events.SwapListenersForTest()
	defer restore()
	var got []string
	events.RegisterListener(events.Message{}, func(e events.Event) events.ListenerReturn {
		if m, ok := e.(events.Message); ok && m.UserId == 1 {
			got = append(got, m.Text)
		}
		return events.Continue
	})
	events.RegisterListener(events.Quest{}, func(e events.Event) events.ListenerReturn {
		q := e.(events.Quest)
		if u := users.GetByUserId(q.UserId); u != nil {
			u.Character.GiveQuestToken(q.QuestToken)
			u.SendText(messaging.CategorySystem, reshowBanner+q.QuestToken)
			u.SendText(messaging.CategoryLoot, "You receive 10 gold!")
			if q.QuestToken == "9-end" {
				// A reward quest (Rewards.QuestId), queued from inside the
				// handler as HandleQuestUpdate does: its banner lands on a
				// later turn of the same pass, and the pitch must wait for it.
				events.AddToQueue(events.Quest{UserId: q.UserId, QuestToken: "13-start"})
			}
		}
		return events.Continue
	})
	events.RegisterListener(events.TalkNextOffer{}, conversation.HandleTalkNextOffer)
	events.ProcessEvents()
	return got
}

// #375 P1: the hand-in's completion banner and reward print first, then the
// next offer's pitch, then its Accept / Leave card, and no talk menu.
func TestHandIn_NextOfferPitchFollowsTheCompletionLines(t *testing.T) {
	user, mob, done := fixtureWith(t, nextOfferFixtureYAML)
	defer done()
	defer quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 9, Name: "The Lost Purse", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 12, Name: "The Missing Ledger", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
	)()
	defer events.DrainAllQueuedEventsForTest()
	require.True(t, user.Character.GiveQuestToken("9-start"), "the player is on quest 9")
	giveItem(t, user, 10001)
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	require.True(t, user.RouteMenuInput("1", true), "1 is the hand-in")
	require.False(t, user.HasOpenMenu(), "nothing re-opens before the card")

	got := deliveredWithNextOffer(t)
	banner := indexOf(got, reshowBanner+"9-end")
	chained := indexOf(got, reshowBanner+"13-start")
	reward := indexOf(got, "You receive 10 gold!") // the last one, the chained quest's
	ask := indexOf(got, "Is there more I can do?")
	pitch := indexOf(got, "My ledger went missing too.")
	card := indexOf(got, "Accept")
	require.GreaterOrEqual(t, banner, 0, "%q", got)
	require.Greater(t, chained, banner, "%q", got)
	require.Greater(t, reward, chained, "%q", got)
	require.Greater(t, ask, reward, "the player's line follows every completion line: %q", got)
	require.Greater(t, pitch, ask, "%q", got)
	require.Greater(t, card, pitch, "the card follows the pitch: %q", got)
	require.Equal(t, len(got)-1, card, "the card prints last: %q", got)
	require.Contains(t, got[card], "Leave")
	require.Equal(t, -1, indexOf(got, "Talking with"), "no talk menu opens: %q", got)

	// The open menu is the card: Accept grants the next quest.
	require.True(t, user.HasOpenMenu())
	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	require.Equal(t, []string{"12-start"}, questTokensFor(1))
}
