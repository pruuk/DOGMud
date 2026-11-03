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
