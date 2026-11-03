package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

func TestQuest75EndsAtBrennan(t *testing.T) {
	q := loadShippedQuest(t, "75-the_surveyors_report.yaml", 75)
	require.Equal(t, []string{"start", "survey", "intro", "notes", "end"}, newLinearPlayer(q).steps)

	w := newQuestWalker(q).at("intro")
	w.notify("topic_heard", EventDetails{MobId: 9524, Topic: "testimony"})
	require.Equal(t, "notes", w.p.current, "Reth's own word moves the step")
	require.Contains(t, w.ctx.givenItems, 40160, "and he hands over the map")

	brennan := loadShippedDialogue(t, "greenford", "9516.yaml")
	in := brennan.FindTopic("notes_in")
	require.NotNil(t, in)
	require.Equal(t, dialogue.KindHandIn, in.Kind)
	require.Equal(t, []string{"75-notes"}, in.QuestRequired)
	require.Equal(t, "75-end", in.GrantsQuest)
}

func TestQuest77TheTruth(t *testing.T) {
	const stones, fragment = 6397, 40176
	q := loadShippedQuest(t, "77-the_truth.yaml", 77)
	require.Equal(t, []string{"start", "read", "end"}, newLinearPlayer(q).steps)
	require.NoError(t, loadShippedQuest(t, "77-the_truth.yaml", 77).Validate())

	w := newQuestWalker(q).at("start")
	w.notify("command", EventDetails{Command: "search", RoomId: stones})
	require.Equal(t, "read", w.p.current)
	require.Equal(t, []int{fragment}, w.ctx.givenItems)

	keeper := loadShippedDialogue(t, "eastern_highlands", "9553.yaml")
	in := keeper.FindTopic("truth_in")
	require.NotNil(t, in)
	require.Equal(t, dialogue.KindHandIn, in.Kind)
	require.Equal(t, []string{"77-read"}, in.QuestRequired)
	require.Equal(t, "77-end", in.GrantsQuest)
	offer := keeper.FindTopic("truth_offer")
	require.NotNil(t, offer)
	require.Equal(t, dialogue.KindOffer, offer.Kind)
	require.Equal(t, "77-start", offer.GrantsQuest)

	t.Run("a player inside who never took the quest is pointed at the Keeper and moves nothing", func(t *testing.T) {
		w := newQuestWalker(q)
		w.notify("command", EventDetails{Command: "search", RoomId: stones})
		require.Empty(t, w.p.current)
		require.Empty(t, w.ctx.givenItems)
		require.Len(t, w.ctx.sentTexts, 1)
		require.Contains(t, w.ctx.sentTexts[0], "The one at the door might.")
	})

	t.Run("reading the stones plays once, and no later search gives a second fragment", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "search", RoomId: stones})
		require.Len(t, w.ctx.sentTexts, 1, "the first search is the reading, and only the reading")
		w.notify("command", EventDetails{Command: "search", RoomId: stones})
		require.Equal(t, []int{fragment}, w.ctx.givenItems)
		require.Len(t, w.ctx.sentTexts, 1)
	})

	t.Run("the oracle-stones answer a reader, and only a reader", func(t *testing.T) {
		look := EventDetails{RoomId: stones, Verb: "look", Noun: "oracle-stones", RoomNoun: "oracle stones"}
		w := newQuestWalker(q).at("start")
		w.notify("room_interact", look)
		require.Empty(t, w.ctx.sentTexts)
		w = newQuestWalker(q).at("read")
		w.notify("room_interact", look)
		require.Len(t, w.ctx.sentTexts, 1)
	})
}
