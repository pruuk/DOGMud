package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 tier 1, hook 3: accepting an offer for an item the player
// already holds (a carriesItem offer) queues TalkMenuAgain instead of opening
// the menu at once, so the grant and the step start run before the menu is
// built. An ordinary offer re-shows the menu at once, as before.
const menuAgainCarriesYAML = `mobid: 1
zone: TestZone
topics:
  - key: hunt
    kind: offer
    quest: 7
    label: "The Wolf Hunt"
    playerLine: "Do you want the wolves dealt with?"
    text: "Bring me three pelts."
    carriesItem: 10001
    questExcluded: [7-start, 7-end]
    grantsQuest: 7-start
  - key: bones
    kind: info
    label: "Why do your bones rattle?"
    text: "Old age."
`

const menuAgainPlainYAML = `mobid: 1
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
  - key: bones
    kind: info
    label: "Why do your bones rattle?"
    text: "Old age."
`

func acceptHunt(t *testing.T, src string) (userHasMenu bool, queued []string, done func()) {
	t.Helper()
	user, mob, cleanup := fixtureWith(t, src)
	user.Character.Items = append(user.Character.Items, items.Item{ItemId: 10001})
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true), "the offer is topic 1")
	events.DrainAllQueuedEventsForTest()
	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	return user.HasOpenMenu(), events.QueuedTypesInOrderForTest(), func() {
		events.DrainAllQueuedEventsForTest()
		cleanup()
	}
}

func TestOffer_AcceptingAHeldItemsOfferQueuesTheMenuInsteadOfOpeningIt(t *testing.T) {
	open, queued, done := acceptHunt(t, menuAgainCarriesYAML)
	defer done()
	require.False(t, open, "the menu waits for the quest chain")
	var relevant []string
	for _, ty := range queued {
		if ty == "Quest" || ty == "TalkMenuAgain" {
			relevant = append(relevant, ty)
		}
	}
	require.Equal(t, []string{"Quest", "TalkMenuAgain"}, relevant, "the grant runs before the menu")
}

func TestOffer_AcceptingAnOrdinaryOffersReshowsTheMenuAtOnce(t *testing.T) {
	open, queued, done := acceptHunt(t, menuAgainPlainYAML)
	defer done()
	require.True(t, open, "positive control: an ordinary offer opens the menu itself")
	require.NotContains(t, queued, "TalkMenuAgain")
}

func TestHandleTalkMenuAgain_OpensTheTalkMenu(t *testing.T) {
	user, mob, cleanup := fixtureWith(t, menuAgainPlainYAML)
	defer cleanup()
	require.False(t, user.HasOpenMenu())

	conversation.HandleTalkMenuAgain(events.TalkMenuAgain{UserId: user.UserId, MobInstanceId: mob.InstanceId})

	require.True(t, user.HasOpenMenu(), "the talk menu is shown again")
	require.Contains(t, drained(user.UserId), "Talking with Skeleton")
}
