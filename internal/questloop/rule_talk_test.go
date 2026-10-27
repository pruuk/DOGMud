package questloop

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// talkWorld is goodWorld plus Nell (9200), whose info topic "rumours"
// grants the given token.
func talkWorld(grant string) World {
	w := goodWorld()
	w.Dialogue = append(w.Dialogue, &dialogue.DialogueFile{MobId: 9200, Zone: "pothole_coulee", Topics: []dialogue.Topic{
		{Key: "rumours", Kind: dialogue.KindInfo, Label: "Rumours", Text: "t",
			Gates:   dialogue.Gates{QuestRequired: []string{"53-start"}},
			Effects: dialogue.Effects{GrantsQuest: grant}},
	}})
	return w
}

func talkFindings(fs []Finding) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Rule == "talk-on-quest" {
			out = append(out, f)
		}
	}
	return out
}

// A topic granting a mid-quest step is a talk step the quest file does not
// own: one finding, keyed by the topic.
func TestTalkOnQuest_TopicGrantingAMidStep(t *testing.T) {
	got := talkFindings(Validate(talkWorld("53-firstkill")))
	require.Len(t, got, 1, "%v", got)
	require.Equal(t, "talk-on-quest 53 mob 9200 topic rumours", got[0].Key())
	require.Equal(t, "a topic grants a mid-quest step; give the quest a topic_heard trigger for it instead", got[0].Msg)
}

// The offer (start) and the hand-in (end) are the topics that may grant;
// a revoke grants nothing.
func TestTalkOnQuest_StartEndAndRevokeAreNotFindings(t *testing.T) {
	require.Empty(t, talkFindings(Validate(goodWorld())), "offer grants start, hand-in grants end")
	for _, tok := range []string{"53-start", "53-end", "-53-firstkill"} {
		require.Empty(t, talkFindings(Validate(talkWorld(tok))), tok)
	}
}
