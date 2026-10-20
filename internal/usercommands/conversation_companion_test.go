package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/events"
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

	_, _, ok = conversation.CompanionTopic(user, mob, "e")
	require.False(t, ok, "a word in several labels matches nothing")
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
	room := mustRoom(t, user)
	_ = room

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
