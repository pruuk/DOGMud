package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
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

// Spec 4.1 step 3: with several next offers nothing is pitched; the talk
// menu is re-shown and lists each as a [!] line.
func TestHandIn_SeveralNextOffersReshowTheMenu(t *testing.T) {
	const twoOffers = nextOfferFixtureYAML + `  - key: seal
    kind: offer
    quest: 15
    label: "The Broken Seal"
    playerLine: "What about the seal?"
    text: "Someone broke my seal."
    questRequired: [9-end]
    questExcluded: [15-start, 15-end]
    grantsQuest: 15-start
`
	user, mob, done := fixtureWith(t, twoOffers)
	defer done()
	defer quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 9, Name: "The Lost Purse", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 12, Name: "The Missing Ledger", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 15, Name: "The Broken Seal", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
	)()
	defer events.DrainAllQueuedEventsForTest()
	require.True(t, user.Character.GiveQuestToken("9-start"))
	giveItem(t, user, 10001)
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	require.True(t, user.RouteMenuInput("1", true), "1 is the hand-in")

	got := deliveredWithNextOffer(t)
	// The player's lines also show in the menu as sublines; only the spoken
	// form ("You say") or the NPC's pitch would mean a pitch ran.
	for _, said := range []string{`saytext">Is there more`, `saytext">What about the seal`, "My ledger went missing too.", "Someone broke my seal."} {
		require.Equal(t, -1, indexOf(got, said), "no pitch (%s): %q", said, got)
	}
	menuAt := indexOf(got, "Talking with")
	require.Greater(t, menuAt, indexOf(got, "My thanks."), "the talk menu is re-shown: %q", got)
	require.True(t, user.HasOpenMenu())
	for _, label := range []string{"The Missing Ledger", "The Broken Seal"} {
		var line string
		for _, l := range strings.Split(got[menuAt], "\n") {
			if strings.Contains(l, label) {
				line = l
			}
		}
		require.Contains(t, line, "[!]", "%s is a [!] line: %q", label, got[menuAt])
	}
}

// Spec 4.1: the NPC left the room before the queued pitch ran. Nothing is
// said and nothing opens.
func TestHandIn_NextOfferAfterTheNpcLeftPitchesNothing(t *testing.T) {
	user, mob, done := fixtureWith(t, nextOfferFixtureYAML)
	defer done()
	defer quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 9, Name: "The Lost Purse", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 12, Name: "The Missing Ledger", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
	)()
	defer events.DrainAllQueuedEventsForTest()
	require.True(t, user.Character.GiveQuestToken("9-start"))
	require.True(t, user.Character.GiveQuestToken("9-end"))
	events.AddToQueue(events.TalkNextOffer{UserId: user.UserId, MobInstanceId: mob.InstanceId, TopicKey: "ledger"}, events.TalkNextOfferPriority)
	home := mob.Character.RoomId
	mob.Character.RoomId = home + 1000
	defer func() { mob.Character.RoomId = home }()

	got := deliveredWithNextOffer(t)
	require.Equal(t, -1, indexOf(got, "Is there more I can do?"), "no line: %q", got)
	require.Equal(t, -1, indexOf(got, "My ledger went missing too."), "no pitch: %q", got)
	require.Equal(t, -1, indexOf(got, "Accept"), "no card: %q", got)
	require.False(t, user.HasOpenMenu())
}

// A repeatable offer, plus an info topic so the talk menu still opens once
// the offer is hidden.
const repeatOfferFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: wash
    kind: offer
    quest: 14
    label: "Cull the Wash"
    playerLine: "Any work for me?"
    text: "The bandits are back in the wash."
    questExcluded: [14-start, 14-end]
    grantsQuest: 14-start
  - key: bones
    kind: info
    label: "Why do your bones rattle?"
    text: "Old age."
`

// #375 phase 2 wiring: the real card carries the body (objective, reward,
// repeat line), and a repeatable on cooldown leaves the talk menu and its
// marker (the production PlayerState.QuestOpen).
func TestTalk_RepeatableOfferCardAndCooldownWiring(t *testing.T) {
	user, mob, done := fixtureWith(t, repeatOfferFixtureYAML)
	defer done()
	q := &quests.Quest{QuestId: 14, Name: "Cull the Wash", Repeatable: true, CooldownRounds: 50,
		Steps: []quests.QuestStep{
			{Id: "start", Description: "Thin out the bandits in the wash."},
			{Id: "end"},
		},
		Rewards: quests.QuestReward{Gold: 6},
	}
	defer quests.SeedQuestsForTest(q)()
	defer events.DrainAllQueuedEventsForTest()
	room := mustRoom(t, user)

	require.Equal(t, "[!]", conversation.Marker(user.UserId, mob))
	conversation.Talk(user, room, mob)
	require.Contains(t, drained(1), "Cull the Wash", "the offer is in the talk menu")
	require.True(t, user.RouteMenuInput("1", true), "1 is the offer")
	card := drained(1)
	require.Contains(t, card, "Accept")
	require.Contains(t, card, "Thin out the bandits in the wash.")
	require.Contains(t, card, "Reward: 6 gold")
	require.Contains(t, card, "You can take this on again after a while.")
	require.True(t, user.RouteMenuInput("2", true), "2 is Leave")
	drained(1)
	user.CloseMenu(menu.CloseNone)

	user.Character.RecordCompletion(q)
	require.False(t, user.Character.QuestOpen(q), "fixture: on cooldown")
	require.Equal(t, "", conversation.Marker(user.UserId, mob), "no [!] while on cooldown")
	conversation.Talk(user, room, mob)
	menuText := drained(1)
	require.Contains(t, menuText, "Why do your bones rattle?", "the talk menu opened")
	require.NotContains(t, menuText, "Cull the Wash", "the offer is hidden on cooldown")
}

// #375 review: give hands in while a prompt is open, and the card could not
// open over it. The queued pitch must not speak a line the player never
// chose; the offer waits in the talk menu with its [!].
func TestHandIn_NextOfferWithPromptOpenPitchesNothing(t *testing.T) {
	user, mob, done := fixtureWith(t, nextOfferFixtureYAML)
	defer done()
	defer quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 9, Name: "The Lost Purse", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 12, Name: "The Missing Ledger", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
	)()
	defer events.DrainAllQueuedEventsForTest()
	require.True(t, user.Character.GiveQuestToken("9-start"), "the player is on quest 9")
	itm := giveItem(t, user, 10001)
	user.StartPrompt("build", "")
	defer user.ClearPrompt()

	require.True(t, conversation.Give(user, mustRoom(t, user), mob, itm))
	got := deliveredWithNextOffer(t)
	require.GreaterOrEqual(t, indexOf(got, "My thanks."), 0, "the hand-in played: %q", got)
	require.Equal(t, -1, indexOf(got, "Is there more I can do?"), "no line the player never chose: %q", got)
	require.Equal(t, -1, indexOf(got, "My ledger went missing too."), "no pitch: %q", got)
	require.Equal(t, -1, indexOf(got, "Accept"), "no card: %q", got)
	require.False(t, user.HasOpenMenu())
	require.NotNil(t, user.GetPrompt(), "the prompt is untouched")
	require.Equal(t, "[!]", conversation.Marker(user.UserId, mob), "the offer still waits")
}

// The same with a menu gate holding (asleep) when the queued pitch runs.
func TestHandIn_NextOfferWhileGatedPitchesNothing(t *testing.T) {
	user, mob, done := fixtureWith(t, nextOfferFixtureYAML)
	defer done()
	defer quests.SeedQuestsForTest(
		&quests.Quest{QuestId: 9, Name: "The Lost Purse", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 12, Name: "The Missing Ledger", Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
	)()
	defer events.DrainAllQueuedEventsForTest()
	require.True(t, user.Character.GiveQuestToken("9-start"))
	user.Character.GiveQuestToken("9-end")
	events.AddToQueue(events.TalkNextOffer{UserId: user.UserId, MobInstanceId: mob.InstanceId, TopicKey: "ledger"}, events.TalkNextOfferPriority)
	const sleepId = 9457
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		sleepId: {ConditionId: sleepId, Name: "Test Sleep", RoundInterval: 1, TriggerCount: 100,
			Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	require.NoError(t, user.Character.AddCondition(sleepId, true))
	reason, _ := user.MenuBlockedReason()
	require.NotEqual(t, menu.CloseNone, reason, "fixture: the gate holds")

	got := deliveredWithNextOffer(t)
	require.Equal(t, -1, indexOf(got, "Is there more I can do?"), "no line the player never chose: %q", got)
	require.Equal(t, -1, indexOf(got, "My ledger went missing too."), "no pitch: %q", got)
	require.False(t, user.HasOpenMenu())
}
