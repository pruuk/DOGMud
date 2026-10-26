package questengine

import (
	"fmt"
	"strings"
	"testing"

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

// The Lore spoke (Pothole Coulee quests 47, 48, 49 and the repeatable 58),
// walked through the real YAML under the owner ruling of 2026-10-17: one
// command per step, steps in order, the report back to the giver only in the
// last step's hint, and the giver's mid-quest greeting naming the current
// step's one command.

// stepHint returns the hint of one step.
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

// requireOnlyFinalStepReports: the report back to the giver ("ask <giver>
// done") belongs to the last step before end, and only there.
func loreOnlyFinalStepReports(t *testing.T, q *QuestDef, giver string) {
	t.Helper()
	report := "type talk " + giver // R20 (#375): talk replaces ask

	last := q.Steps[len(q.Steps)-2].Id
	for _, s := range q.Steps {
		if s.Id == "end" {
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

func TestQuest47FirstWords_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "47-first_words.yaml", 47)
	require.Equal(t, []string{"start", "help", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; keep them so a player mid-quest resumes")
	turnIn := turnInRequires(t, "9153.yaml", "words_turnin")
	require.Equal(t, []string{"47-help"}, turnIn, "the turn-in gates on the last step")
	loreOnlyFinalStepReports(t, q, "wenna")
	require.Equal(t, "Type help.", strings.TrimSpace(loreStepHint(t, q, "start")))

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "say", RoomId: 5324})
		require.Equal(t, "start", w.p.current, "another command must not finish the help lesson")
		require.False(t, holdsAll(w.p, turnIn))

		// Owner call (2026-10-17, this spoke): help works anywhere, which is
		// the lesson, so the step is not room-gated.
		w.notify("command", EventDetails{Command: "help", RoomId: 5200})
		require.Equal(t, "help", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on help can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("help")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("Wenna answers mid-quest questions about the lesson", func(t *testing.T) {
		f := "47-first_words.yaml"
		checkTopicSteps(t, "wenna", "9153.yaml", []topicStep{
			{f, 47, "start", "type help", []string{"30-end"}, "words_progress", false},
			{f, 47, "help", "", []string{"30-end"}, "words_turnin", true},
		})
	})
}

func TestQuest48StandingStones_OneCommandPerStep(t *testing.T) {
	const green, circle = 5335, 5333
	q := loadShippedQuest(t, "48-the_standing_stones.yaml", 48)
	require.Equal(t, []string{"start", "taunt", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; keep them so a player mid-quest resumes")
	turnIn := turnInRequires(t, "9157.yaml", "stones_turnin")
	require.Equal(t, []string{"48-taunt"}, turnIn, "the turn-in gates on the last step")
	loreOnlyFinalStepReports(t, q, "hale")
	require.Contains(t, strings.ToLower(loreStepHint(t, q, "start")), "type taunt garrow")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "taunt", RoomId: circle})
		require.Equal(t, "start", w.p.current, "a taunt away from the green must not count")
		require.False(t, holdsAll(w.p, turnIn))

		w.notify("command", EventDetails{Command: "taunt", RoomId: green})
		require.Equal(t, "taunt", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on taunt can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("taunt")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("Hale's greeting and answers follow the step", func(t *testing.T) {
		f := "48-the_standing_stones.yaml"
		checkTopicSteps(t, "hale", "9157.yaml", []topicStep{
			{f, 48, "start", "type taunt garrow", []string{"47-end"}, "stones_progress", false},
			{f, 48, "taunt", "", []string{"47-end"}, "stones_turnin", true},
		})
	})
}

// searchFloor is `search floor` in a room: since #375 phase 3a quests 49
// and 58 listen for a search of the Reliquary floor, where the disc lies.
func searchFloor(roomId int) EventDetails {
	return EventDetails{RoomId: roomId, Verb: "search", Noun: "floor", RoomNoun: "floor"}
}

func TestQuest49OldShrine_OneCommandPerStep(t *testing.T) {
	const reliquary, nave = 5343, 5342
	q := loadShippedQuest(t, "49-the_old_shrine.yaml", 49)
	require.Equal(t, []string{"start", "find", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; keep them so a player mid-quest resumes")
	turnIn := turnInRequires(t, "9160.yaml", "shrine_turnin")
	require.Equal(t, []string{"49-find"}, turnIn, "the turn-in gates on the last step")
	loreOnlyFinalStepReports(t, q, "sere")
	start := strings.ToLower(loreStepHint(t, q, "start"))
	require.Contains(t, start, "type search")
	require.NotContains(t, start, "look at", "looking at the find is not a step; it belongs in the description")
	require.NotContains(t, start, "go up", "the Reliquary is west then north of the shrine, not up")
	// The search lesson (#375 phase 3a, owner ruling 9): search the one
	// thing that looks disturbed, or the whole room, slower and less sure.
	// Only the first is typed; the second is named, not asked for.
	require.Contains(t, start, "type search floor")
	require.Contains(t, start, "whole room")
	require.Contains(t, start, "less sure")
	require.Contains(t, start, "type help search", "the lesson points at its help page")
	offer := topicHints(t, "9160.yaml", "shrine_offer")
	require.Contains(t, offer, "type search floor", "Sere's offer teaches the same two routes")
	require.Contains(t, offer, "whole room")

	t.Run("the Reliquary hints at the floor and never instructs", func(t *testing.T) {
		desc := strings.ToLower(roomDescription(t, "pothole_coulee", 5343))
		require.Contains(t, desc, "flagstone floor")
		require.NotContains(t, desc, "search", "a room hints at its secret; the step hint teaches the command (ruling 7)")
	})

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("room_interact", searchFloor(nave))
		require.Equal(t, "start", w.p.current, "a search outside the Reliquary must not count")
		require.False(t, holdsAll(w.p, turnIn))

		look := searchFloor(reliquary)
		look.Verb = "look"
		w.notify("room_interact", look)
		require.Equal(t, "start", w.p.current, "looking at the floor is not searching it")

		w.notify("room_interact", searchFloor(reliquary))
		require.Equal(t, "find", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on find can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("find")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("Sere's greeting and answers follow the step", func(t *testing.T) {
		f := "49-the_old_shrine.yaml"
		checkTopicSteps(t, "sere", "9160.yaml", []topicStep{
			{f, 49, "start", "type search", []string{"47-end", "48-end"}, "shrine_progress", false},
			{f, 49, "find", "", []string{"47-end", "48-end"}, "shrine_turnin", true},
		})
	})
}

func TestQuest58ReadingTheStones_FinishesWhereItSays(t *testing.T) {
	const reliquary = 5343
	q := loadShippedQuest(t, "58-reading_the_stones.yaml", 58)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)

	// The search in the Reliquary finishes the quest on the spot, so nothing
	// may send the player back to Sere, or narrate Sere listening.
	start := strings.ToLower(loreStepHint(t, q, "start"))
	require.Contains(t, start, "type search")
	for _, back := range []string{"come back", "return", "sere"} {
		require.NotContains(t, start, back, "the hint must not send the player back: the search finishes the quest")
	}
	require.NotContains(t, strings.ToLower(q.Rewards.PlayerMessage), "listens",
		"the reward plays in the Reliquary; Sere is not there to listen")
	require.NotContains(t, strings.ToLower(q.Rewards.RoomMessage), "sere",
		"the reward's room line plays in the Reliquary, where Sere is not")
	for _, tr := range q.Triggers {
		for _, a := range tr.Actions {
			require.NotContains(t, strings.ToLower(a.SendText), "bring word",
				"the search finishes the quest; there is no word to bring")
		}
	}

	w := newQuestWalker(q).at("start")
	w.notify("room_interact", searchFloor(5342))
	require.Equal(t, "start", w.p.current, "a search outside the Reliquary must not count")
	w.notify("room_interact", searchFloor(reliquary))
	require.Equal(t, "end", w.p.current)

	t.Run("Sere's greeting names the search while it is open", func(t *testing.T) {
		// A real character on 49-end holds every earlier 48 and 49 token too.
		checkTopicSteps(t, "sere", "9160.yaml", []topicStep{
			{"58-reading_the_stones.yaml", 58, "start", "type search",
				[]string{"47-end", "48-start", "48-taunt", "48-end", "49-start", "49-find", "49-end"}, "stones_loop_progress", false},
		})
	})
}
