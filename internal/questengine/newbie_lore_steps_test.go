package questengine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// topicHints is the lowercased hint of one topic in a Pothole Coulee
// dialogue file.
func topicHints(t *testing.T, file, key string) string {
	t.Helper()
	tp := loadCouleeTopics(t, file).FindTopic(key)
	require.NotNil(t, tp, "%s has no topic %s", file, key)
	return strings.ToLower(tp.Hints)
}

// roomDescription is a shipped room's description.
func roomDescription(t *testing.T, zoneFolder string, roomId int) string {
	t.Helper()
	var room struct {
		Description string `yaml:"description"`
	}
	require.NoError(t, yaml.Unmarshal(repoFile(t, "_datafiles", "world", "dogmud", "rooms", zoneFolder, fmt.Sprintf("%d.yaml", roomId)), &room))
	return room.Description
}

// The lore line (Pothole Coulee quests 47, 48, 49 and the repeatable 58)
// under #375 phase 7: giver to giver. Toke offers 47 and Wenna takes it;
// Wenna offers 48 and Hale takes it; Hale offers 49 and Sere takes it; Sere
// keeps the repeatable 58 and the bridge 95. One command per step; every
// quest after the first opens with the walk to the giver who takes it.

func loreStepHint(t *testing.T, q *QuestDef, step string) string {
	t.Helper()
	for _, s := range q.Steps {
		if s.Id == step {
			return s.Hint
		}
	}
	t.Fatalf("quest %d has no step %q", q.QuestId, step)
	return ""
}

// loreOnlyFinalStepReports: the report back to the giver belongs to the last
// step before end. A meeting step (start, when the next step is met) also
// teaches talk, to the giver the player is walking to, and is left out.
func loreOnlyFinalStepReports(t *testing.T, q *QuestDef, giver string) {
	t.Helper()
	report := "type talk " + giver
	meeting := len(q.Steps) > 1 && q.Steps[1].Id == "met"
	last := q.Steps[len(q.Steps)-2].Id
	for _, s := range q.Steps {
		if s.Id == "end" || (meeting && s.Id == "start") {
			continue
		}
		has := strings.Contains(strings.ToLower(s.Hint), report)
		if s.Id == last {
			require.True(t, has, "quest %d last step %q must tell the player to type %s: %q", q.QuestId, s.Id, report, s.Hint)
		} else {
			require.False(t, has, "quest %d step %q previews the report; it belongs to the last step only: %q", q.QuestId, s.Id, s.Hint)
		}
	}
}

// loreOfferOn: the quest's offer sits on this lore giver and on no other.
func loreOfferOn(t *testing.T, id int, file string) {
	t.Helper()
	for _, f := range []string{"9153.yaml", "9157.yaml", "9160.yaml"} {
		has := false
		for _, tp := range loadCouleeTopics(t, f).Topics {
			if tp.Kind == dialogue.KindOffer && tp.Quest == id {
				has = true
			}
		}
		require.Equal(t, f == file, has, "quest %d's offer belongs on %s only (on %s: %v)", id, file, f, has)
	}
}

func TestQuest47FirstWords_GiverToGiver(t *testing.T) {
	q := loadShippedQuest(t, "47-first_words.yaml", 47)
	require.Equal(t, []string{"start", "met", "help", "end"}, newLinearPlayer(q).steps,
		"start and help keep their ids so a saved player resumes; met is the meeting with Wenna")
	turnIn := turnInRequires(t, "9153.yaml", "words_turnin")
	require.Equal(t, []string{"47-help"}, turnIn, "the turn-in gates on the last step")
	loreOnlyFinalStepReports(t, q, "wenna")
	require.Equal(t, "Type help.", strings.TrimSpace(loreStepHint(t, q, "met")))
	require.Contains(t, squash(loreStepHint(t, q, "start")), "east, then north six times to the commons yard")
	require.Equal(t, 5324, foldingWalk(t, foldingHubSquare, "east", "north", "north", "north", "north", "north", "north"))

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "help", RoomId: 5200})
		require.Equal(t, "start", w.p.current, "help before Wenna's lesson does not count")
		w.notify("topic_heard", EventDetails{MobId: 9153, Topic: "words_lesson"})
		require.Equal(t, "met", w.p.current)
		w.notify("command", EventDetails{Command: "say", RoomId: 5324})
		require.Equal(t, "met", w.p.current, "another command must not finish the help lesson")
		// Help works anywhere, which is the lesson (owner 2026-10-17).
		w.notify("command", EventDetails{Command: "help", RoomId: 5200})
		require.Equal(t, "help", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on help can turn in", func(t *testing.T) {
		require.True(t, holdsAll(newQuestWalker(q).at("help").p, turnIn))
	})

	t.Run("Wenna's greeting and answers follow the step", func(t *testing.T) {
		f := "47-first_words.yaml"
		pre := []string{"30-end", "31-end"}
		checkMeetStep(t, "wenna", "9153.yaml", f, 47, "words_lesson", "type help", pre...)
		checkTopicSteps(t, "wenna", "9153.yaml", []topicStep{
			{f, 47, "met", "type help", pre, "words_progress", false},
			{f, 47, "help", "", pre, "words_turnin", true},
		})
	})

	// #690: nothing the player did was called "the reckoning"; the reckoning
	// is Hale's carved bone.
	t.Run("Wenna never calls the help the reckoning", func(t *testing.T) {
		raw := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", "9153.yaml")))
		require.NotContains(t, raw, "reckoning")
	})
}
