package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// The seeded Skeleton (MobId 1, instance 100, zone "TestZone") gets this
// dialogue file. Quest tokens are not registered; PlayerState.HasQuest
// reads the character's progress, so tests grant tokens by hand.
const talkFixtureYAML = `mobid: 1
zone: TestZone
opener:
  text: "Rattle."
topics:
  - key: bones
    kind: info
    label: "Why do your bones rattle?"
    text: "Old age."
  - key: secret
    kind: info
    label: "Tell me a secret."
    text: "The crypt key is under the altar."
    marked: true
  - key: deeper
    kind: info
    label: "What lies under the altar?"
    text: "Stairs."
    heardRequired: [secret]
`

const offerFixtureYAML = `mobid: 1
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
`

const merchantFixtureYAML = `mobid: 2
zone: TestZone
opener:
  text: "Wares for sale."
topics:
  - key: prices
    kind: info
    label: "What do you charge?"
    text: "Fair prices, friend."
`

func installDialogue(t *testing.T, src string) func() {
	t.Helper()
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict([]byte(src), &df))
	return dialogue.InstallForTest(&df)
}

// fixtureWith seeds the registries and gives the Skeleton the dialogue in
// src. The seeded instance carries no Zone (only its template does), while
// every live instance carries its template's zone, so the fixture sets it:
// dialogue.Load keys on the instance's MobId and Zone.
func fixtureWith(t *testing.T, src string) (*users.UserRecord, *mobs.Mob, func()) {
	t.Helper()
	cleanup := seedAllRegistries()
	undo := installDialogue(t, src)
	user, _ := getTestUserAndRoom(t)
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Zone = "TestZone"
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	events.DrainQueuedQuestsForTest()
	return user, mob, func() { undo(); cleanup() }
}

func talkFixture(t *testing.T) (*users.UserRecord, *mobs.Mob, func()) {
	return fixtureWith(t, talkFixtureYAML)
}

func offerFixture(t *testing.T) (*users.UserRecord, *mobs.Mob, func()) {
	return fixtureWith(t, offerFixtureYAML)
}

func drained(uid int) string { return strings.Join(events.DrainQueuedMessagesForTest(uid), "\n") }

func mustRoom(t *testing.T, u *users.UserRecord) *rooms.Room {
	t.Helper()
	r := rooms.LoadRoom(u.Character.RoomId)
	require.NotNil(t, r)
	return r
}

func questTokensFor(uid int) []string {
	var out []string
	for _, q := range events.DrainQueuedQuestsForTest() {
		if q.UserId == uid {
			out = append(out, q.QuestToken)
		}
	}
	return out
}

// The room-line assertions below need user 2 to see user 1 by name.
func TestTalk_FixtureObserverHasFullSight(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	room := mustRoom(t, user)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(users.GetByUserId(2).Character, room))
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(user.Character, room))
}

func TestTalk_OpenerThenPrivateMenu(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	room := mustRoom(t, user)
	conversation.Talk(user, room, mob)

	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "Rattle.")
	require.Contains(t, mine, "Talking with Skeleton")
	require.Contains(t, mine, "Why do your bones rattle?")
	require.NotContains(t, mine, "What lies under the altar?", "gated on heardRequired")
	require.Contains(t, theirs, "approaches")
	require.Contains(t, theirs, "Rattle.")
	require.NotContains(t, theirs, "Talking with", "observers never see the menu")
	require.True(t, user.HasOpenMenu())
}

func TestTalk_InfoChoiceSpeaksBothLinesMarksHeardAndReshows(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	drained(2)

	// Menu order: bones, secret. "2" is the marked secret.
	require.True(t, user.RouteMenuInput("2", true))
	mine, theirs := drained(1), drained(2)
	require.Contains(t, theirs, "Tell me a secret.")
	require.Contains(t, theirs, "The crypt key is under the altar.")
	require.Contains(t, mine, `You say, "`)
	require.True(t, user.Character.HasHeard("1.secret"))
	require.True(t, user.HasOpenMenu(), "the menu is re-shown after an answer")
	require.Contains(t, mine, "What lies under the altar?", "heardRequired now met")
}

func TestTalk_StillValidFailsWhenTheNpcLeaves(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	mob.Character.RoomId = 2
	require.True(t, user.RouteMenuInput("1", true))
	require.False(t, user.HasOpenMenu())
	require.False(t, user.Character.HasHeard("1.bones"))
}

func TestOffer_PitchThenCardLeaveGrantsNothing(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	drained(2)

	require.True(t, user.RouteMenuInput("1", true))
	mine, theirs := drained(1), drained(2)
	require.Contains(t, theirs, "Do you want the wolves dealt with?")
	require.Contains(t, theirs, "Bring me three pelts.")
	require.Contains(t, mine, "Accept")
	require.Contains(t, mine, "Leave")
	require.NotContains(t, theirs, "Accept", "the card is private")

	require.True(t, user.RouteMenuInput("", true), "Enter answers the card")
	require.NotContains(t, questTokensFor(1), "7-start")
	require.True(t, user.HasOpenMenu(), "the talk menu came back")
	require.Contains(t, drained(1), "Talking with Skeleton")
}

func TestOffer_AcceptGrantsOnce(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true))
	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	require.Equal(t, []string{"7-start"}, questTokensFor(1))
	require.False(t, user.HasOpenMenu(), "the only other topic wants an item the player lacks, so no menu is left to re-show")

	user.Character.QuestProgress = map[int]string{7: "start"}
	drained(1)
	conversation.Talk(user, mustRoom(t, user), mob)
	require.NotContains(t, drained(1), "The Wolf Hunt")
}

// GiveQuest only queues the token; the quest engine records it on a later
// event pass. The menu re-shown straight after Accept must still treat the
// quest as taken, or it offers the quest the player just accepted.
func TestOffer_ReshowAfterAcceptHidesTheOffer(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true))
	drained(1)
	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	require.False(t, user.Character.HasQuest("7-start"), "precondition: the grant is still queued")
	require.NotContains(t, drained(1), "The Wolf Hunt")
}

func TestOffer_AcceptAfterEligibilityLostGrantsNothing(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true))
	user.Character.QuestProgress = map[int]string{7: "start"}
	drained(1)
	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	require.Contains(t, drained(1), "That offer is no longer open.")
	require.NotContains(t, questTokensFor(1), "7-start")
}

func TestTalk_SecondNpcReplacesTheMenu(t *testing.T) {
	user, skeleton, done := talkFixture(t)
	defer done()
	defer installDialogue(t, merchantFixtureYAML)()
	merchant := &mobs.Mob{
		MobId:      2,
		Zone:       "TestZone",
		InstanceId: 101,
		HomeRoomId: 1,
		Character: characters.Character{
			Name:       "Merchant",
			RoomId:     1,
			Health:     50,
			Conditions: conditions.New(),
			Cooldowns:  map[string]int{},
		},
	}
	merchant.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(101, merchant)
	defer mobs.SetInstanceForTest(101, nil)
	room := mustRoom(t, user)
	room.AddMob(101)
	defer room.RemoveMob(101)

	conversation.Talk(user, room, skeleton)
	conversation.Talk(user, room, merchant)
	mine := drained(1)
	require.True(t, user.HasOpenMenu())
	last := mine[strings.LastIndex(mine, "Talking with"):]
	require.True(t, strings.HasPrefix(last, "Talking with Merchant"), "the open menu is the Merchant's")

	require.True(t, user.RouteMenuInput("1", true))
	theirs := drained(2)
	require.Contains(t, theirs, "Fair prices, friend.")
	require.True(t, user.Character.HasHeard("2.prices"))
	require.False(t, user.Character.HasHeard("1.bones"))
}

func TestHandIn_LineThenHandoverThenReply(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	require.True(t, user.Character.StoreItem(items.New(10001)))
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	drained(2)

	// Offers and hand-ins in authored order: hunt (1), purse (2).
	require.True(t, user.RouteMenuInput("2", true))
	mine := drained(1)
	say := strings.Index(mine, `You say, "<ansi fg="saytext">Here is the purse`)
	give := strings.Index(mine, "You give the")
	reply := strings.Index(mine, "My thanks.")
	require.GreaterOrEqual(t, say, 0, mine)
	require.Greater(t, give, say, "handover after the player's line")
	require.Greater(t, reply, give, "the NPC replies after the handover")
	for _, itm := range user.Character.GetAllBackpackItems() {
		require.NotEqual(t, 10001, itm.ItemId, "the purse left the backpack")
	}
	require.Equal(t, []string{"9-end"}, questTokensFor(1))
	require.Contains(t, drained(2), "gave their")
}
