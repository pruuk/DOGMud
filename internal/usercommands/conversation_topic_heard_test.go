package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/stretchr/testify/require"
)

// Every way a topic plays raises topic_heard (#375 phase 3b slice 1): a menu
// choice, a companion's ask on the owner's behalf, and an accepted offer.
// The seeded Skeleton is MobId 1.

const topicHeardPayout = 7

// payOnTopicHeard registers, on a fresh engine, one quest whose topic_heard
// trigger on the Skeleton's topic key pays topicHeardPayout gold. Gold, not
// a grant, so a second raise of the same event shows (the engine's guard
// skips a duplicate grant).
func payOnTopicHeard(t *testing.T, questId int, key string) {
	t.Helper()
	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: questId, Name: "topic heard probe",
		Steps: []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event: "topic_heard", Mob: 1, Topic: key,
			Actions: []questengine.ActionDef{{GiveGold: topicHeardPayout}},
		}},
	})
}

// grantOnTopicHeard registers a quest whose topic_heard trigger on the
// Skeleton's key grants questId-start, and returns that token.
func grantOnTopicHeard(t *testing.T, questId int, key string) string {
	t.Helper()
	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	token := fmt.Sprintf("%d-start", questId)
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: questId, Name: "topic heard probe",
		Steps: []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event: "topic_heard", Mob: 1, Topic: key,
			Actions: []questengine.ActionDef{{Grant: token}},
		}},
	})
	return token
}

// A menu choice raises topic_heard exactly once.
func TestTopicHeard_MenuChoiceRaisesOnce(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	payOnTopicHeard(t, 999421, "bones")
	user.Character.Gold = 0

	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true), "1 is bones")
	require.True(t, user.Character.HasHeard("1.bones"), "the topic played")
	require.Equal(t, topicHeardPayout, user.Character.Gold, "one topic_heard, not two")
}

// A companion asking on the owner's behalf plays the topic for the owner,
// so the owner's topic_heard step advances, as it does when the owner asks.
func TestTopicHeard_CompanionAskAdvancesTheOwnersStep(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	token := grantOnTopicHeard(t, 999422, "secret")
	user.Character.ClearQuestToken(token)
	t.Cleanup(func() { user.Character.ClearQuestToken(token) })

	require.True(t, conversation.AskForOwner(1, mob.InstanceId, "secret", true))
	require.True(t, user.Character.HasHeard("1.secret"), "the topic played")
	require.True(t, user.Character.HasQuest(token), "the owner's topic_heard trigger fired")
}

// Accepting an offer plays its topic, so it raises topic_heard; leaving it
// raises nothing.
func TestTopicHeard_AcceptedOfferRaises(t *testing.T) {
	user, mob, done := offerFixture(t)
	defer done()
	payOnTopicHeard(t, 999423, "hunt")
	user.Character.Gold = 0

	conversation.Talk(user, mustRoom(t, user), mob)
	require.True(t, user.RouteMenuInput("1", true), "1 is the hunt offer")
	require.Equal(t, 0, user.Character.Gold, "a pitch is not a played topic")
	require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
	require.Equal(t, topicHeardPayout, user.Character.Gold, "the accepted offer raised topic_heard once")
}
