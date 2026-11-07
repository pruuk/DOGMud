package questengine

import (
	"strings"
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

func TestQuest102TheKnock(t *testing.T) {
	const door = 5530
	q := loadShippedQuest(t, "102-the_knock.yaml", 102)
	require.Equal(t, []string{"start", "behind", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, q.Validate())

	open := func(step string) bool {
		w := newQuestWalker(q).at(step)
		return !w.e.WouldFire("room_gate", EventDetails{UserId: 1, RoomId: door}, w.p)
	}
	require.False(t, open(""), "no knock, no door")
	require.True(t, open("start"))
	require.True(t, open("behind"))
	require.True(t, open("end"), "the dealer stays open")

	w := newQuestWalker(q).at("start")
	w.notify("room_enter", EventDetails{RoomId: door})
	require.Equal(t, "behind", w.p.current)
	line := "three months at minimum before the new one's producing at volume, if she survives the first month"
	require.Contains(t, w.ctx.sentTexts[0], line)
	n := len(w.ctx.sentTexts)
	w.notify("room_enter", EventDetails{RoomId: door})
	require.Len(t, w.ctx.sentTexts, n, "the overheard line plays once")

	// A player with no knock, teleported in, overhears nothing and gains no step.
	w = newQuestWalker(q).at("")
	w.notify("room_enter", EventDetails{RoomId: door})
	require.Equal(t, "", w.p.current)
	require.Empty(t, w.ctx.sentTexts)

	require.Empty(t, q.Rewards.Gold)
	for _, s := range q.Steps {
		require.LessOrEqual(t, len(s.Hint), 200, s.Id)
	}
}

// The door's refusal is what a look and a step both read, so it says nothing of
// hands on latches, and it is true for a payer who shelved the quest as well as
// for a stranger. The knock itself is taught when the gold has gone, again on a
// resume short of the room, and not to one who has already been through.
func TestQuest102DoorAndKnockLines(t *testing.T) {
	const door = 5530
	q := loadShippedQuest(t, "102-the_knock.yaml", 102)

	w := newQuestWalker(q).at("")
	w.notify("room_gate", EventDetails{RoomId: door})
	require.Len(t, w.ctx.sentTexts, 1)
	refusal := w.ctx.sentTexts[0]
	require.Contains(t, refusal, "barred from the other side")
	require.Contains(t, refusal, "knock")
	for _, banned := range []string{"You try", "you do not know", "taught you", "latch"} {
		require.NotContains(t, refusal, banned)
	}

	taught := func(step string) string {
		w := newQuestWalker(q).at(step)
		w.notify("quest_granted", EventDetails{QuestToken: "102-start"})
		return strings.Join(w.ctx.sentTexts, "\n")
	}
	require.Contains(t, taught("start"), "three short, then one")
	require.Empty(t, taught("behind"), "they are through already")
}
