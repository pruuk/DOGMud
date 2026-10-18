package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// The Lore spoke (Pothole Coulee quests 47, 48, 49 and the repeatable 58),
// walked through the real YAML under the owner ruling of 2026-10-17: one
// command per step, steps in order, the report back to the giver only in the
// last step's hint, and the giver's mid-quest greeting naming the current
// step's one command.

// loreDialogue is the slice of a shipped dialogue file these tests read: the
// root variants (greetings) and the tree nodes, with their quest gates.
type loreDialogue struct {
	Tree struct {
		Root struct {
			Variants []struct {
				QuestRequired []string `yaml:"questRequired"`
				QuestExcluded []string `yaml:"questExcluded"`
				Hints         string   `yaml:"hints"`
			} `yaml:"variants"`
		} `yaml:"root"`
		Nodes []struct {
			Id            string   `yaml:"id"`
			Triggers      []string `yaml:"triggers"`
			QuestRequired []string `yaml:"questRequired"`
			QuestExcluded []string `yaml:"questExcluded"`
			GrantsQuest   string   `yaml:"grantsQuest"`
			Hints         string   `yaml:"hints"`
		} `yaml:"nodes"`
	} `yaml:"tree"`
}

func loadLoreDialogue(t *testing.T, file string) *loreDialogue {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", file)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var df loreDialogue
	require.NoError(t, yaml.Unmarshal(data, &df))
	return &df
}

// gateOpen mirrors dialogue.checkQuestGate's token half: every required token
// held, no excluded token held.
func loreGateOpen(p *linearPlayer, required, excluded []string) bool {
	for _, r := range required {
		if !p.HasQuest(r) {
			return false
		}
	}
	for _, x := range excluded {
		if p.HasQuest(x) {
			return false
		}
	}
	return true
}

// greetingHint is the hint of the first root variant open to the player, the
// way dialogue.Greet picks it.
func (df *loreDialogue) greetingHint(p *linearPlayer) string {
	for _, v := range df.Tree.Root.Variants {
		if loreGateOpen(p, v.QuestRequired, v.QuestExcluded) {
			return v.Hints
		}
	}
	return ""
}

// answer returns the id and hint of the first tree node that answers `ask
// <npc> <topic>`, the way dialogue.TreeAdvance picks it (substring match on a
// trigger, then the quest gate). An empty id means the question falls through
// to the patterns.
func (df *loreDialogue) answer(p *linearPlayer, topic string) (string, string) {
	topic = strings.ToLower(topic)
	for _, n := range df.Tree.Nodes {
		hit := false
		for _, tr := range n.Triggers {
			if strings.Contains(topic, strings.ToLower(tr)) {
				hit = true
				break
			}
		}
		if hit && loreGateOpen(p, n.QuestRequired, n.QuestExcluded) {
			return n.Id, n.Hints
		}
	}
	return "", ""
}

// holding gives the walker's player the earlier quests' tokens a real player
// on this quest already holds, so the giver's "not ready" greeting stays shut.
func (w *questWalker) loreHolding(tokens ...string) *questWalker {
	for _, tok := range tokens {
		w.p.fullMockPlayer.quests[tok] = true
	}
	return w
}

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
	report := "ask " + giver + " done"
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
		df := loadLoreDialogue(t, "9153.yaml")
		w := newQuestWalker(q).at("start").loreHolding("30-end")
		require.Equal(t, "Type help.", df.greetingHint(w.p))
		for _, topic := range []string{"done", "ready", "quest", "task", "help", "how"} {
			id, hint := df.answer(w.p, topic)
			require.NotEmpty(t, id, "ask wenna %s mid-quest falls through to the catch-all", topic)
			require.NotEqual(t, "words_turnin", id)
			require.Contains(t, strings.ToLower(hint), "type help", "ask wenna %s must point at the current step", topic)
		}
		w = newQuestWalker(q).at("help").loreHolding("30-end")
		require.Equal(t, "Type ask wenna done.", df.greetingHint(w.p))
		id, _ := df.answer(w.p, "done")
		require.Equal(t, "words_turnin", id)
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
		df := loadLoreDialogue(t, "9157.yaml")
		w := newQuestWalker(q).at("start").loreHolding("47-end")
		hint := strings.ToLower(df.greetingHint(w.p))
		require.Contains(t, hint, "type taunt garrow")
		require.NotContains(t, hint, "done", "the mid-quest greeting must not preview the report")
		for _, topic := range []string{"done", "ready", "quest", "task", "taunt", "garrow", "how", "help"} {
			id, hint := df.answer(w.p, topic)
			require.NotEmpty(t, id, "ask hale %s mid-quest falls through to the patterns", topic)
			require.NotEqual(t, "stones_turnin", id)
			require.Contains(t, strings.ToLower(hint), "type taunt garrow", "ask hale %s must point at the current step", topic)
		}
		w = newQuestWalker(q).at("taunt").loreHolding("47-end")
		require.Equal(t, "type ask hale done.", strings.ToLower(df.greetingHint(w.p)))
		id, _ := df.answer(w.p, "done")
		require.Equal(t, "stones_turnin", id)
	})
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

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "search", RoomId: nave})
		require.Equal(t, "start", w.p.current, "a search outside the Reliquary must not count")
		require.False(t, holdsAll(w.p, turnIn))

		w.notify("command", EventDetails{Command: "search", RoomId: reliquary})
		require.Equal(t, "find", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on find can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("find")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("Sere's greeting and answers follow the step", func(t *testing.T) {
		df := loadLoreDialogue(t, "9160.yaml")
		w := newQuestWalker(q).at("start").loreHolding("47-end", "48-end")
		hint := strings.ToLower(df.greetingHint(w.p))
		require.Contains(t, hint, "type search")
		require.NotContains(t, hint, "done", "the mid-quest greeting must not preview the report")
		for _, topic := range []string{"done", "ready", "quest", "task", "search", "disc", "floor", "how", "help"} {
			id, hint := df.answer(w.p, topic)
			require.NotEmpty(t, id, "ask sere %s mid-quest falls through to the patterns", topic)
			require.NotEqual(t, "shrine_turnin", id)
			require.Contains(t, strings.ToLower(hint), "type search", "ask sere %s must point at the current step", topic)
		}
		w = newQuestWalker(q).at("find").loreHolding("47-end", "48-end")
		require.Equal(t, "type ask sere done.", strings.ToLower(df.greetingHint(w.p)))
		id, _ := df.answer(w.p, "done")
		require.Equal(t, "shrine_turnin", id)
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
	w.notify("command", EventDetails{Command: "search", RoomId: 5342})
	require.Equal(t, "start", w.p.current, "a search outside the Reliquary must not count")
	w.notify("command", EventDetails{Command: "search", RoomId: reliquary})
	require.Equal(t, "end", w.p.current)

	t.Run("Sere's greeting names the search while it is open", func(t *testing.T) {
		df := loadLoreDialogue(t, "9160.yaml")
		w := newQuestWalker(q).at("start")
		// A real character on 49-end holds every earlier 48 and 49 token too.
		for _, tok := range []string{"47-end", "48-start", "48-taunt", "48-end", "49-start", "49-find", "49-end"} {
			w.p.fullMockPlayer.quests[tok] = true
		}
		hint := strings.ToLower(df.greetingHint(w.p))
		require.Contains(t, hint, "type search")
	})
}
