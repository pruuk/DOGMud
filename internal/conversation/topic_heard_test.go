package conversation

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	topicHeardMobId  = 9112
	topicHeardUserId = 7311
	freeQuest        = 999311
	paidQuest        = 999312
	freeToken        = "999311-start"
	paidToken        = "999312-start"
)

// topicHeardScene registers one topic_heard trigger per topic key (each
// granting its own token) on a fresh engine and returns a played-topic
// driver. play is driven directly: the player is muted (no room say), the
// topics carry no NPC text, so only the user and an empty room are needed.
func topicHeardScene(t *testing.T) (*users.UserRecord, func(t *dialogue.Topic) bool) {
	t.Helper()
	mudlog.SetupLogger(nil, "", "", false) // the engine logs; an unset logger panics
	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	// One quest per topic: a quest's first token must be its start step.
	for id, topic := range map[int]string{freeQuest: "free", paidQuest: "paid"} {
		questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
			QuestId: id, Name: "topic heard probe " + topic,
			Steps: []questengine.QuestStep{{Id: "start"}},
			Triggers: []questengine.TriggerDef{
				{Event: "topic_heard", Mob: topicHeardMobId, Topic: topic, Actions: []questengine.ActionDef{{Grant: fmt.Sprintf("%d-start", id)}}},
			},
		})
	}

	user := users.NewTestUser(topicHeardUserId, "asker", "Asker", 7311)
	user.Muted = true
	user.Character.Gold = 0
	room := &rooms.Room{RoomId: 5200}
	mob := &mobs.Mob{MobId: topicHeardMobId, InstanceId: 31100}
	df := &dialogue.DialogueFile{MobId: topicHeardMobId}
	ps := PlayerState(user)
	for _, tok := range []string{freeToken, paidToken} {
		user.Character.ClearQuestToken(tok)
		tok := tok
		t.Cleanup(func() { user.Character.ClearQuestToken(tok) })
	}
	return user, func(topic *dialogue.Topic) bool {
		_, applied := play(user, room, mob, df, topic, ps, &lineBuffer{user: user})
		return applied
	}
}

// play raises topic_heard once the topic's effects landed; a refused price
// raises nothing. The free topic is the positive control: the same trigger
// path fires for it, so the paid topic's silence is a real result.
func TestPlayRaisesTopicHeardOnlyWhenTheTopicLands(t *testing.T) {
	user, run := topicHeardScene(t)

	paid := &dialogue.Topic{Key: "paid", Kind: dialogue.KindInfo, Label: "Pay", Effects: dialogue.Effects{ChargesGold: 50}}
	require.False(t, run(paid), "the player has no gold: the charge is refused")
	require.False(t, user.Character.HasQuest(paidToken), "a refused charge raises no topic_heard")

	free := &dialogue.Topic{Key: "free", Kind: dialogue.KindInfo, Label: "Ask"}
	require.True(t, run(free))
	require.True(t, user.Character.HasQuest(freeToken), "the played topic's trigger fired (positive control)")
	require.False(t, user.Character.HasQuest(paidToken), "another topic's trigger did not")

	user.Character.Gold = 50
	require.True(t, run(paid))
	require.True(t, user.Character.HasQuest(paidToken), "once the price is paid the same topic fires")
}
