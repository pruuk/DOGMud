package questworld

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// holds is a progress check that holds exactly the given tokens.
func holds(toks ...string) func(string) bool {
	return func(tok string) bool {
		for _, x := range toks {
			if x == tok {
				return true
			}
		}
		return false
	}
}

// A topic of quest 70 gated on 63-start hands out item 900. For quest 63 it
// is a source from 63-start on, not one only 63's later steps open; for 70
// it is 70's own; for any other quest it is outside.
func TestTopicOwners_GateOnAnotherQuestsStepOpensFromThatStep(t *testing.T) {
	tp := dialogue.Topic{Key: "spare", Quest: 70,
		Gates: dialogue.Gates{QuestRequired: []string{"63-start"}}, Effects: dialogue.Effects{GivesItem: 900}}
	ss := []itemSource{{where: "mob 1 topic spare", owners: topicOwners(tp)}}

	require.False(t, available(ss, 63, holds()), "before 63-start the topic is closed")
	require.True(t, available(ss, 63, holds("63-start")), "from 63-start the topic is open")
	require.False(t, available(ss, 70, holds("63-start")), "the topic is quest 70's own")
	require.True(t, available(ss, 5, holds()), "for another quest it is outside")
}

// Quest 20's vellahandoff shape: a topic gated on 20-brindle hands out the
// journal and grants 20-vella_journal in the same choice. Taking it moves
// the quest, so the item is not available at a step before vella_journal
// even though the gate is held there.
func TestTopicOwners_TopicThatGrantsAStepGatesItsItemOnThatStep(t *testing.T) {
	tp := dialogue.Topic{Key: "vellahandoff", Kind: dialogue.KindInfo,
		Gates:   dialogue.Gates{QuestRequired: []string{"20-brindle"}},
		Effects: dialogue.Effects{GrantsQuest: "20-vella_journal", GivesItem: 40061}}
	ss := []itemSource{{where: "mob 355 topic vellahandoff", owners: topicOwners(tp)}}

	require.False(t, available(ss, 20, holds("20-start", "20-brindle", "20-kingfisher")),
		"taking the topic grants vella_journal, so it is no source before that step")
	require.True(t, available(ss, 20, holds("20-start", "20-brindle", "20-kingfisher", "20-vella_journal")))
	require.True(t, available(ss, 5, holds()))
}

// A replacement topic of quest 20 gated on its own step 20-journal is open
// from that step; an ungated topic of quest 20 is 20's own at every step.
func TestTopicOwners_OwnQuestGateOpensFromThatStep(t *testing.T) {
	lost := dialogue.Topic{Key: "lost", Quest: 20, Gates: dialogue.Gates{QuestRequired: []string{"20-journal"}}}
	plain := dialogue.Topic{Key: "plain", Quest: 20}
	gated := []itemSource{{where: "lost", owners: topicOwners(lost)}}
	own := []itemSource{{where: "plain", owners: topicOwners(plain)}}

	require.False(t, available(gated, 20, holds("20-start")))
	require.True(t, available(gated, 20, holds("20-start", "20-journal")))
	require.False(t, available(own, 20, holds("20-start", "20-journal")))
	require.True(t, available(own, 21, holds()))
}
