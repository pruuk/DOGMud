package questengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuest64TheSkip(t *testing.T) {
	const odile, floor = 9801, 5625
	q := loadShippedQuest(t, "64-the_skip.yaml", 64)
	require.Equal(t, []string{"start", "found", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())

	open := func(step string) bool {
		w := newQuestWalker(q).at(step)
		return !w.e.WouldFire("room_gate", EventDetails{UserId: 1, RoomId: floor}, w.p)
	}
	require.False(t, open(""), "no quest, no stair")
	require.True(t, open("start"))
	require.True(t, open("found"))
	require.False(t, open("end"), "her hiding place closes with her story")

	w := newQuestWalker(q).at("start")
	w.notify("topic_heard", EventDetails{MobId: odile, Topic: "who"})
	require.Equal(t, "start", w.p.current)
	w.notify("topic_heard", EventDetails{MobId: odile, Topic: "q64_found"})
	require.Equal(t, "found", w.p.current)

	require.NotZero(t, q.Steps[0].Destination)
	require.Empty(t, q.Rewards.Gold, "each ending pays on its own hand-in")
	require.Empty(t, q.Rewards.ItemId, "the wafer comes with the report, not with every ending")

	for _, s := range q.Steps {
		require.LessOrEqual(t, len(s.Hint), 200, s.Id)
	}
}
