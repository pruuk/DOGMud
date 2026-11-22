package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// An offer and a hand-in beside an info topic, so the menu has something
// to re-show after either one.
const reshowFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: hunt
    kind: offer
    quest: 7
    label: "The Wolf Hunt"
    playerLine: "Do you want the wolves dealt with?"
    text: "Bring me three pelts."
    questExcluded: [7-start, 7-end]
    grantsQuest: 7-start
  - key: purse
    kind: handin
    quest: 9
    label: "The Lost Purse"
    playerLine: "Here is the purse you were looking for."
    text: "My thanks."
    requiresItem: 10001
    questExcluded: [9-end]
    grantsQuest: 9-end
  - key: bones
    kind: info
    label: "Why do your bones rattle?"
    text: "Old age."
`

const reshowBanner = "QUEST BANNER for "

// deliveredInOrder runs the event queue the way the game loop does and
// returns every line user 1 receives, in delivery order (runTalkQueue).
func deliveredInOrder(t *testing.T) []string {
	t.Helper()
	return runTalkQueue(t, nil)
}

// runTalkQueue runs the event queue the way the game loop does, with the real
// talk listeners (TalkMenuAgain, TalkNextOffer) and a stand-in quest engine
// that lands each granted token, then prints a banner and a reward line for
// it, as HandleQuestUpdate does; then calls also, when set, for any chain the
// test needs. It records every line user 1 receives, in delivery order.
// HandleQuestUpdate itself cannot run here: package hooks imports
// usercommands, so this package's tests cannot import hooks (the root
// package's coulee spine walk runs it for real).
func runTalkQueue(t *testing.T, also func(events.Quest)) []string {
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
	events.RegisterListener(events.TalkMenuAgain{}, conversation.HandleTalkMenuAgain)
	events.RegisterListener(events.TalkNextOffer{}, conversation.HandleTalkNextOffer)
	events.RegisterListener(events.Quest{}, func(e events.Event) events.ListenerReturn {
		q := e.(events.Quest)
		if u := users.GetByUserId(q.UserId); u != nil {
			u.Character.GiveQuestToken(q.QuestToken)
			u.SendText(messaging.CategorySystem, reshowBanner+q.QuestToken)
			u.SendText(messaging.CategoryLoot, "You receive 10 gold!")
			if also != nil {
				also(q)
			}
		}
		return events.Continue
	})
	events.ProcessEvents()
	return got
}

func indexOf(lines []string, sub string) int {
	idx := -1
	for i, l := range lines {
		if strings.Contains(l, sub) {
			idx = i
		}
	}
	return idx
}

// After Accept or a hand-in, the quest banner and reward lines print on a
// later event pass than the choice. The re-shown menu printed before them,
// so they pushed it off screen (playtest seats A and B, #375). The menu must
// come after them.
func TestTalk_ReshownMenuComesAfterTheQuestBanner(t *testing.T) {
	t.Run("accept", func(t *testing.T) {
		user, mob, done := fixtureWith(t, reshowFixtureYAML)
		defer done()
		conversation.Talk(user, mustRoom(t, user), mob)
		require.True(t, user.RouteMenuInput("1", true), "1 is the offer")
		drained(1)
		require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
		require.False(t, user.HasOpenMenu(), "the menu is queued behind the grant (#940)")

		got := deliveredInOrder(t)
		require.True(t, user.HasOpenMenu(), "the menu is re-shown for input once the queue has run")
		banner, menuAt := indexOf(got, reshowBanner+"7-start"), indexOf(got, "Talking with")
		require.GreaterOrEqual(t, banner, 0, "%q", got)
		require.Greater(t, menuAt, indexOf(got, "You receive 10 gold!"), "the menu printed before the banner and rewards: %q", got)
	})
	t.Run("hand-in", func(t *testing.T) {
		user, mob, done := fixtureWith(t, reshowFixtureYAML)
		defer done()
		giveItem(t, user, 10001)
		conversation.Talk(user, mustRoom(t, user), mob)
		drained(1)
		require.True(t, user.RouteMenuInput("2", true), "2 is the hand-in")

		got := deliveredInOrder(t)
		banner, menuAt := indexOf(got, reshowBanner+"9-end"), indexOf(got, "Talking with")
		require.GreaterOrEqual(t, banner, 0, "%q", got)
		require.Greater(t, menuAt, indexOf(got, "You receive 10 gold!"), "the menu printed before the banner and rewards: %q", got)
	})
}
