package usercommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
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

func backpackHas(u *users.UserRecord, itemId int) bool {
	for _, itm := range u.Character.GetAllBackpackItems() {
		if itm.ItemId == itemId {
			return true
		}
	}
	return false
}

// spec 4 item 1: the gate runs before anyone speaks. A blocked player gets
// one private line and the room hears nothing.
func TestTalk_BusyRefusesPrivatelyBeforeAnyoneSpeaks(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	user.Character.Activity = activity.NewMachine()
	require.NoError(t, user.Character.Activity.Inner().TransitionTo(activity.Crafting, state.TransitionReason{}))

	conversation.Talk(user, mustRoom(t, user), mob)
	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "You are too busy to start a conversation.")
	require.NotContains(t, mine, "Rattle.")
	require.Empty(t, theirs, "the room hears nothing")
	require.False(t, user.HasOpenMenu())
}

func TestTalk_OpenPromptRefusesPrivately(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	user.StartPrompt("build", "")
	defer user.ClearPrompt()

	conversation.Talk(user, mustRoom(t, user), mob)
	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "Finish what you have open before you start a conversation.")
	require.Empty(t, theirs)
	require.False(t, user.HasOpenMenu())
}

const rewardFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: purse
    kind: handin
    quest: 9
    label: "The Lost Purse"
    playerLine: "Here is the purse you were looking for."
    text: "Take this for your trouble."
    requiresItem: 10001
    givesItem: 30001
    questExcluded: [9-end]
    grantsQuest: 9-end
`

// The callbacks' private lines (here "You receive") come after the handover
// and before the NPC's reply.
func TestHandIn_RewardLineFollowsTheHandover(t *testing.T) {
	user, mob, done := fixtureWith(t, rewardFixtureYAML)
	defer done()
	require.True(t, user.Character.StoreItem(items.New(10001)))
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)

	require.True(t, user.RouteMenuInput("1", true))
	mine := drained(1)
	say := strings.Index(mine, `You say, "<ansi fg="saytext">Here is the purse`)
	give := strings.Index(mine, "You give the")
	receive := strings.Index(mine, "You receive")
	reply := strings.Index(mine, "Take this for your trouble.")
	require.GreaterOrEqual(t, say, 0, mine)
	require.Greater(t, give, say, mine)
	require.Greater(t, receive, give, "the reward line follows the handover")
	require.Greater(t, reply, receive, "the NPC replies last")
	require.True(t, backpackHas(user, 30001))
}

func TestHandIn_QueuesTheItemLoss(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	require.True(t, user.Character.StoreItem(items.New(10001)))
	conversation.Talk(user, mustRoom(t, user), mob)
	events.DrainQueuedItemOwnershipForTest(1)

	require.True(t, user.RouteMenuInput("2", true))
	got := events.DrainQueuedItemOwnershipForTest(1)
	require.Len(t, got, 1)
	require.False(t, got[0].Gained)
	require.Equal(t, 10001, got[0].Item.ItemId)
}

const chargeFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: map
    kind: info
    label: "Will you trade your map for my sword?"
    text: "Here it is."
    requiresItem: 10001
    chargesGold: 50
`

func TestTalk_RefusedChargeGivesNoHandoverAndNoReply(t *testing.T) {
	user, mob, done := fixtureWith(t, chargeFixtureYAML)
	defer done()
	user.Character.Gold = 0
	require.True(t, user.Character.StoreItem(items.New(10001)))
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	drained(2)

	require.True(t, user.RouteMenuInput("1", true))
	mine, theirs := drained(1), drained(2)
	require.Contains(t, theirs, "Will you trade your map for my sword?")
	require.NotContains(t, theirs, "gave their")
	require.NotContains(t, theirs, "Here it is.")
	require.NotContains(t, mine, "You give the")
	require.NotContains(t, mine, "Here it is.")
	require.Contains(t, mine, "You don't have enough gold for that.")
	require.True(t, backpackHas(user, 10001), "the item stays with the player")
	require.False(t, user.Character.HasHeard("1.map"))
}

const itemOfferFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: ferry
    kind: offer
    quest: 5
    label: "The Ferry Fare"
    playerLine: "Can I ride the ferry?"
    text: "Leave your sword with me as surety."
    requiresItem: 10001
    questExcluded: [5-start]
    grantsQuest: 5-start
`

func TestOffer_AcceptWithAnItemPrintsTheHandover(t *testing.T) {
	user, mob, done := fixtureWith(t, itemOfferFixtureYAML)
	defer done()
	require.True(t, user.Character.StoreItem(items.New(10001)))
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true))
	drained(1)
	drained(2)

	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	require.Contains(t, drained(1), `You give the <ansi fg="item">Iron Sword</ansi>`)
	require.Contains(t, drained(2), "gave their")
	require.False(t, backpackHas(user, 10001))
	require.Equal(t, []string{"5-start"}, questTokensFor(1))
}

func TestTalk_DarkRoomEndsTheConversation(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	drained(2)

	user.Character.RoomId = 2
	mob.Character.RoomId = 2
	dark := rooms.LoadRoom(2)
	require.NotNil(t, dark)
	require.NotEqual(t, messaging.SightFull, messaging.ParticipantSight(user.Character, dark),
		"precondition: room 2 is too dark to see names")

	require.True(t, user.RouteMenuInput("1", true))
	require.False(t, user.HasOpenMenu())
	require.False(t, user.Character.HasHeard("1.bones"))
	require.NotContains(t, drained(1), "Old age.")
}

// Spec 4 item 1: the NPC must be visible by name. A player who makes out
// shapes only can aim talk at a figure, but gets one private refusal naming
// the light before anyone speaks: no approach line, no opener, no menu
// (#375 review I1). The lit control opens the menu.
func TestTalkCommand_AtAShapeRefusesBeforeAnyoneSpeaks(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	_, room := aimSceneSeeded(t, aimShapes)
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(user.Character, room),
		"precondition: the talker makes out shapes only")

	_, err := Talk("2.shape", user, room, 0)
	require.NoError(t, err)
	mine, theirs := aimTold(1), aimTold(2)
	require.Equal(t, 1, strings.Count(mine, conversation.TalkShapesLine), "one refusal: %q", mine)
	require.NotContains(t, mine, "Rattle.", "the NPC spoke its opener")
	require.NotContains(t, mine, "Skeleton", "the refusal named a shape")
	require.Empty(t, theirs, "the room heard an approach or an opener")
	require.False(t, user.HasOpenMenu())

	// The conversation entry point refuses the same way for any caller.
	conversation.Talk(user, room, mob)
	require.Contains(t, aimTold(1), conversation.TalkShapesLine)
	require.Empty(t, aimTold(2))
	require.False(t, user.HasOpenMenu())
}

func TestTalkCommand_AtAShapeLitControlOpensTheMenu(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	_, room := aimSceneSeeded(t, aimFull)
	_, err := Talk("skeleton", user, room, 0)
	require.NoError(t, err)
	require.True(t, user.HasOpenMenu())
	require.NotContains(t, aimTold(1), conversation.TalkSightLine)
}

const behaviorFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: ring
    kind: info
    label: "Will you ring the bell?"
    text: "I will."
    behavior: true
  - key: knock
    kind: info
    label: "Will you knock on the door?"
    text: "I will."
    behavior: true
`

func TestTalk_BehaviorTopicFiresTheTreeWithItsKey(t *testing.T) {
	user, mob, done := fixtureWith(t, behaviorFixtureYAML)
	defer done()
	tree := `tree:
  type: sequence
  event: player_topic
  children:
    - type: condition
      check: topic_is
      topic: ring
    - type: action
      do: set_state
      key: rung
      value: 1
`
	path := filepath.Join(t.TempDir(), "1-skeleton.yaml")
	require.NoError(t, os.WriteFile(path, []byte(tree), 0644))
	require.NoError(t, behaviortree.GetEngine().LoadTree(int(mob.MobId), path))
	st := behaviortree.EnsureBTreeState(mob)
	st.Set("rung", 0)

	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("2", true), "knock")
	require.Equal(t, 0, st.GetInt("rung"), "knock carries its own key, not ring")
	// A behavior topic closes the menu (R18): talk again for the next choice.
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true), "ring")
	require.Equal(t, 1, st.GetInt("rung"))
}

func TestTalk_BehaviorTopicClosesTheMenu(t *testing.T) {
	user, mob, done := fixtureWith(t, behaviorFixtureYAML)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.HasOpenMenu())
	drained(1)

	require.True(t, user.RouteMenuInput("1", true), "ring")
	require.False(t, user.HasOpenMenu(), "a behavior topic closes the talk menu (R18)")
	require.NotContains(t, drained(1), "Talking with", "the menu is not re-shown")
}

func TestTalk_NoDialogueFileNodsAndOpensNoMenu(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Zone = "TestZone"
	require.Nil(t, dialogue.Load(1, "TestZone"), "precondition: no file for the Skeleton")
	drained(1)
	drained(2)
	events.DrainQueuedInputsForTest(100)

	conversation.Talk(user, room, mob)
	mine, theirs := drained(1), drained(2)
	require.Contains(t, theirs, "approaches")
	require.NotContains(t, theirs, "says,")
	require.NotContains(t, mine, "Talking with")
	require.False(t, user.HasOpenMenu())
	require.Contains(t, events.DrainQueuedInputsForTest(100), "emote nods.")
}

// A choice whose effects landed (heard set, grants) refreshes the GMCP
// markers once; a pitch or a refused choice does not.
func TestTalk_MarkersRefreshOncePerSuccessfulChoice(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	events.DrainQueuedNpcMarkersChangedForTest(1)

	require.True(t, user.RouteMenuInput("2", true))
	require.Len(t, events.DrainQueuedNpcMarkersChangedForTest(1), 1, "info choice")
}

func TestOffer_MarkersRefreshOnAcceptOnly(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	events.DrainQueuedNpcMarkersChangedForTest(1)

	require.True(t, user.RouteMenuInput("1", true))
	require.Empty(t, events.DrainQueuedNpcMarkersChangedForTest(1), "the pitch changes nothing")
	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	require.Len(t, events.DrainQueuedNpcMarkersChangedForTest(1), 1, "accept")
}

func TestTalk_RefusedChoiceDoesNotRefreshMarkers(t *testing.T) {
	user, mob, done := fixtureWith(t, chargeFixtureYAML)
	defer done()
	user.Character.Gold = 0
	require.True(t, user.Character.StoreItem(items.New(10001)))
	conversation.Talk(user, mustRoom(t, user), mob)
	events.DrainQueuedNpcMarkersChangedForTest(1)

	require.True(t, user.RouteMenuInput("1", true))
	require.Empty(t, events.DrainQueuedNpcMarkersChangedForTest(1))
}

// Charmed NPCs offer no topics: the approach line and a nod, nothing more.
func TestTalk_CharmedMobOnlyNods(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	mob.Character.Charmed = characters.NewCharm(2, characters.CharmPermanent, "")
	events.DrainQueuedInputsForTest(100)

	conversation.Talk(user, mustRoom(t, user), mob)
	mine, theirs := drained(1), drained(2)
	require.Contains(t, theirs, "approaches")
	require.NotContains(t, theirs, "Rattle.")
	require.NotContains(t, mine, "Rattle.")
	require.NotContains(t, mine, "Talking with")
	require.False(t, user.HasOpenMenu())
	require.Contains(t, events.DrainQueuedInputsForTest(100), "emote nods.")
}

// The talk command runs conversation.Talk (#375 Task 19): the opener as
// speech, then the private menu.
func TestTalkCommand_OpensTheMenu(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	_, err := Talk("skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	require.True(t, user.HasOpenMenu())
	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "Talking with Skeleton")
	require.Contains(t, theirs, "Rattle.")
	require.NotContains(t, theirs, "Talking with", "observers never see the menu")
}

// talk opens no menu and nobody speaks when the NPC is asleep (the sleep
// refusal moved from ask to talk).
func TestTalkCommand_SleepingNpcRefuses(t *testing.T) {
	const sleepId = 9456
	user, mob, done := talkFixture(t)
	defer done()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		sleepId: {ConditionId: sleepId, Name: "Test Sleep", RoundInterval: 1, TriggerCount: 100,
			Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	require.NoError(t, mob.Character.AddCondition(sleepId, true))
	_, err := Talk("skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	mine, theirs := drained(1), drained(2)
	require.Contains(t, mine, "is fast asleep.")
	require.False(t, user.HasOpenMenu())
	require.NotContains(t, theirs, "Rattle.")
	require.NotContains(t, theirs, "approaches")
}

func TestTalk_ReshownMenuStaysPrivate(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	conversation.Talk(user, mustRoom(t, user), mob)
	drained(1)
	drained(2)

	require.True(t, user.RouteMenuInput("2", true))
	require.True(t, user.HasOpenMenu())
	theirs := drained(2)
	require.NotContains(t, theirs, "Talking with")
	require.NotContains(t, theirs, "What lies under the altar?")
}
