package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/require"
)

func TestCompanionTopic_MatchesKeyOrAllLabelWords(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()

	key, _, ok := conversation.CompanionTopic(user, mob, "bones")
	require.True(t, ok)
	require.Equal(t, "bones", key)

	key, _, ok = conversation.CompanionTopic(user, mob, "secret tell")
	require.True(t, ok, "every word of the label")
	require.Equal(t, "secret", key)

	key, labels, ok := conversation.CompanionTopic(user, mob, "crypt")
	require.False(t, ok, "the word is in a reply, not in a label")
	require.Empty(t, key)
	require.Equal(t, []string{"Why do your bones rattle?", "Tell me a secret."}, labels,
		"the refusal lists the eligible info labels; the gated one is not among them")

	_, _, ok = conversation.CompanionTopic(user, mob, "se")
	require.False(t, ok, "two letters are not a prefix of secret")
}

const wordsFixtureYAML = `mobid: 1
zone: TestZone
topics:
  - key: mine
    kind: info
    label: "Where is the ore mined?"
    text: "North."
  - key: gold
    kind: info
    label: "Who wants more gold?"
    text: "Everyone."
  - key: vault
    kind: info
    label: "Where is gold kept?"
    text: "Below."
  - key: ferry
    kind: info
    label: "Can you take me across?"
    text: "Aye."
    behavior: true
`

func TestCompanionTopic_WholeWordsAndPrefixes(t *testing.T) {
	user, mob, done := fixtureWith(t, wordsFixtureYAML)
	defer done()

	key, _, ok := conversation.CompanionTopic(user, mob, "ore")
	require.True(t, ok, "ore is a word of the first label only; more does not count")
	require.Equal(t, "mine", key)

	key, _, ok = conversation.CompanionTopic(user, mob, "Mined, ORE!")
	require.True(t, ok, "case and punctuation do not matter")
	require.Equal(t, "mine", key)

	key, _, ok = conversation.CompanionTopic(user, mob, "wan")
	require.True(t, ok, "three letters begin a word")
	require.Equal(t, "gold", key)

	_, _, ok = conversation.CompanionTopic(user, mob, "wa")
	require.False(t, ok, "two letters are not a prefix")

	_, _, ok = conversation.CompanionTopic(user, mob, "gol")
	require.False(t, ok, "two labels hold gold: no single match")

	key, _, ok = conversation.CompanionTopic(user, mob, "vault")
	require.True(t, ok)
	require.Equal(t, "vault", key)
}

func TestCompanionTopic_ExactKeyBeatsSeveralLabelHits(t *testing.T) {
	user, mob, done := fixtureWith(t, wordsFixtureYAML)
	defer done()

	_, _, ok := conversation.CompanionTopic(user, mob, "gold kept")
	require.True(t, ok, "fixture: gold kept is the vault label alone")
	_, _, ok = conversation.CompanionTopic(user, mob, "is")
	require.False(t, ok, "fixture: two letters match nothing")

	key, _, ok := conversation.CompanionTopic(user, mob, "gold")
	require.True(t, ok, "gold is a word of two labels but the key of one")
	require.Equal(t, "gold", key)
}

func TestCompanionTopic_SkipsBehaviorTopics(t *testing.T) {
	user, mob, done := fixtureWith(t, wordsFixtureYAML)
	defer done()

	_, labels, ok := conversation.CompanionTopic(user, mob, "ferry")
	require.False(t, ok)
	require.NotContains(t, labels, "Can you take me across?")
	_, _, ok = conversation.CompanionTopic(user, mob, "take across")
	require.False(t, ok)
}

func TestAskForOwner_BehaviorTopicShrugsAndPlaysNothing(t *testing.T) {
	user, mob, done := fixtureWith(t, wordsFixtureYAML)
	defer done()
	events.DrainQueuedInputsForTest(mob.InstanceId)

	require.True(t, conversation.AskForOwner(1, mob.InstanceId, "ferry", true))
	require.Equal(t, []string{"emote shrugs."}, events.DrainQueuedInputsForTest(mob.InstanceId))
	require.NotContains(t, drained(2), "Aye.")
	require.False(t, user.Character.HasHeard("1.ferry"))
}

func TestAskForOwner_GatedKeyShrugs(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	events.DrainQueuedInputsForTest(mob.InstanceId)

	require.True(t, conversation.AskForOwner(1, mob.InstanceId, "deeper", true), "heardRequired secret is not met")
	require.Equal(t, []string{"emote shrugs."}, events.DrainQueuedInputsForTest(mob.InstanceId))
	require.NotContains(t, drained(2), "Stairs.")
	require.False(t, user.Character.HasHeard("1.deeper"))
}

func TestAskForOwner_CharmedMobIsNotHandled(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	mob.Character.Charmed = characters.NewCharm(2, characters.CharmPermanent, "")

	require.False(t, conversation.AskForOwner(1, mob.InstanceId, "secret", true))
	require.False(t, user.Character.HasHeard("1.secret"))
}

// The owner must be able to hold a conversation: awake and idle, the NPC
// in view at full sight (the talk menu's own rules).
func TestAskForOwner_BusyOwnerHearsNothing(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	user.Character.Activity = activity.NewMachine()
	require.NoError(t, user.Character.Activity.Inner().TransitionTo(activity.Crafting, state.TransitionReason{}))

	require.False(t, conversation.AskForOwner(1, mob.InstanceId, "secret", true))
	require.False(t, user.Character.HasHeard("1.secret"))
	require.NotContains(t, drained(2), "The crypt key")
}

func TestAskForOwner_OwnerInTheDarkHearsNothing(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	room := mustRoom(t, user)
	sky, lamp := room.SkyLight, room.Lamp
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
	defer func() { room.SkyLight, room.Lamp = sky, lamp }()
	require.NotEqual(t, messaging.SightFull, messaging.ParticipantSight(user.Character, room), "fixture: the room must be dark")

	require.False(t, conversation.AskForOwner(1, mob.InstanceId, "secret", true))
	require.False(t, user.Character.HasHeard("1.secret"))
	require.NotContains(t, drained(2), "The crypt key")
}

func TestCompanionTopic_NeverOffersOrHandIns(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()

	for _, what := range []string{"wolf hunt", "hunt", "purse", "lost purse"} {
		_, labels, ok := conversation.CompanionTopic(user, mob, what)
		require.False(t, ok, what)
		require.Empty(t, labels, "no info topics here to list")
	}
}

func TestAskForOwner_PlaysTheInfoTopic(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	require.True(t, conversation.AskForOwner(1, mob.InstanceId, "secret", true))

	mine, theirs := drained(1), drained(2)
	require.Contains(t, theirs, "The crypt key is under the altar.")
	require.Contains(t, mine, "The crypt key is under the altar.")
	require.NotContains(t, mine, "You say", "the companion already spoke; the owner says nothing")
	require.NotContains(t, theirs, "Tell me a secret.")
	require.True(t, user.Character.HasHeard("1.secret"))
	require.False(t, user.HasOpenMenu(), "the owner did not open a menu")
}

func TestAskForOwner_RefusesNonInfoKeys(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	events.DrainQueuedInputsForTest(mob.InstanceId)

	for _, key := range []string{"hunt", "purse", "nothing-here"} {
		require.True(t, conversation.AskForOwner(1, mob.InstanceId, key, true), key)
		require.Equal(t, []string{"emote shrugs."}, events.DrainQueuedInputsForTest(mob.InstanceId), key)
	}
	require.Empty(t, events.DrainQueuedQuestsForTest())
	require.False(t, user.HasOpenMenu())
}

func TestAskForOwner_UnauthorizedOrAbsentDoesNothing(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()

	require.False(t, conversation.AskForOwner(1, mob.InstanceId, "secret", false))
	require.False(t, conversation.AskForOwner(1, 99999, "secret", true))
	require.False(t, user.Character.HasHeard("1.secret"))

	mob.Character.RoomId = user.Character.RoomId + 1000
	require.False(t, conversation.AskForOwner(1, mob.InstanceId, "secret", true), "not in the owner's room")
}
