package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml2 "gopkg.in/yaml.v2"
)

// The Wilderness spoke of Pothole Coulee (Scout Tarn 9136, Hunter Delk 9137):
// quests 41, 42, 43 and 56 under the owner's one-command-per-step ruling
// (2026-10-17). The walks use the controller's helpers in
// newbie_one_command_steps_test.go; the dialogue checks run the real dialogue
// engine (Greet, TreeAdvance, MatchWithFallbackInfo) over the shipped files.

const (
	wildPronghorn  = 9139
	wildAlpha      = 9143
	wildRawMeat    = 40014
	wildSaltPouch  = 40017
	wildDelkCamp   = 5290 // Hunter's Hollow, the cooking_fire station
	wildOpenSteppe = 5284
	wildThornbrake = 5289
)

// wildLoadDialogue parses a shipped Pothole Coulee dialogue file the way the
// game's loader does (yaml.v2, which errors on a bare-scalar list field).
func wildLoadDialogue(t *testing.T, file string) *dialogue.DialogueFile {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", file)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml2.Unmarshal(raw, &df), "%s must load, or the NPC goes mute", file)
	return &df
}

// talker is a player's dialogue view: quest progress from a linearPlayer and
// a set of carried items, with every item handed out recorded.
type wildTalker struct {
	p       *linearPlayer
	carried map[int]bool
	given   []int
	userId  int
}

var wildNextTalkerId = 7000

func wildNewTalker(p *linearPlayer) *wildTalker {
	wildNextTalkerId++ // dialogue memory is global; a fresh id per talker
	return &wildTalker{p: p, carried: map[int]bool{}, userId: wildNextTalkerId}
}

func (tk *wildTalker) state() *dialogue.PlayerState {
	return &dialogue.PlayerState{
		HasQuest:    tk.p.HasQuest,
		HasItem:     func(id int) bool { return tk.carried[id] },
		CarriesItem: func(id int) bool { return tk.carried[id] },
		GiveQuest:   func(string) {},
		GiveItem: func(id int) bool {
			tk.given = append(tk.given, id)
			tk.carried[id] = true
			return true
		},
	}
}

func (tk *wildTalker) greet(df *dialogue.DialogueFile) string {
	_, hints, _ := dialogue.Greet(df, 1, tk.userId, tk.state())
	return hints
}

// ask returns the reply, its hints, and whether a tree node answered. A false
// third value means only a pattern (possibly the catch-all) answered.
func (tk *wildTalker) ask(df *dialogue.DialogueFile, topic string) (string, string, bool) {
	if text, hints, _, ok := dialogue.TreeAdvance(df, 1, tk.userId, topic, tk.state()); ok {
		return text, hints, true
	}
	text, _, _, _ := dialogue.MatchWithFallbackInfo(df, 1, topic, tk.state())
	return text, "", false
}

func wildOneTypedCommand(t *testing.T, hint string) {
	t.Helper()
	n := len(typedCommand.FindAllString(strings.ReplaceAll(hint, darkAside, ""), -1))
	require.Equal(t, 1, n, "a hint names exactly one command: %q", hint)
}

func TestQuest42TheHunt_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "42-the_hunt.yaml", 42)
	// start, hunt, rest and cook are the ids a saved character may hold today.
	require.Equal(t, []string{"start", "track", "hunt", "meat", "rest", "cook", "end"}, newLinearPlayer(q).steps,
		"track, kill, take the meat, sleep, cook, report; the old ids keep their places")
	turnIn := turnInRequires(t, "9137.yaml", "hunt_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		// Nothing out of order counts.
		w.notify("mob_death", EventDetails{MobId: wildPronghorn, RoomId: wildThornbrake})
		w.notify("item_gain", EventDetails{ItemId: wildRawMeat, RoomId: wildThornbrake})
		w.notify("command", EventDetails{Command: "sleep", RoomId: wildDelkCamp})
		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp})
		require.Equal(t, "start", w.p.current, "only the track starts the hunt")

		w.notify("command", EventDetails{Command: "track", RoomId: wildThornbrake})
		require.Equal(t, "track", w.p.current)

		w.notify("command", EventDetails{Command: "sleep", RoomId: wildThornbrake})
		require.Equal(t, "track", w.p.current, "sleep before the kill must not skip it")
		w.notify("mob_death", EventDetails{MobId: wildAlpha, RoomId: wildThornbrake})
		require.Equal(t, "track", w.p.current, "only a pronghorn is the kill")
		w.notify("mob_death", EventDetails{MobId: wildPronghorn, RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current)

		w.notify("command", EventDetails{Command: "sleep", RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current, "sleep before taking the meat must not skip it")
		w.notify("item_gain", EventDetails{ItemId: wildSaltPouch, RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current, "only the meat is the meat")
		w.notify("item_gain", EventDetails{ItemId: wildRawMeat, RoomId: wildThornbrake})
		require.Equal(t, "meat", w.p.current)

		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp})
		require.Equal(t, "meat", w.p.current, "cooking before the rest must not skip it")
		w.notify("command", EventDetails{Command: "sleep", RoomId: wildThornbrake})
		require.Equal(t, "rest", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in waits for the cook")

		w.notify("command", EventDetails{Command: "craft", RoomId: wildThornbrake})
		require.Equal(t, "rest", w.p.current, "a craft away from Delk's fire is not the cook")
		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp})
		require.Equal(t, "cook", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("turn-in is not open before the cook", func(t *testing.T) {
		for _, s := range []string{"start", "track", "hunt", "meat", "rest"} {
			require.False(t, holdsAll(newQuestWalker(q).at(s).p, turnIn), "turn-in must stay shut on %s", s)
		}
	})

	t.Run("player saved on start resumes with the track", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "track", RoomId: wildOpenSteppe})
		require.Equal(t, "track", w.p.current)
	})

	t.Run("player saved on hunt resumes with the meat", func(t *testing.T) {
		w := newQuestWalker(q).at("hunt")
		w.notify("item_gain", EventDetails{ItemId: wildRawMeat, RoomId: wildThornbrake})
		require.Equal(t, "meat", w.p.current)
	})

	t.Run("player saved on rest resumes with the cook", func(t *testing.T) {
		w := newQuestWalker(q).at("rest")
		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp})
		require.Equal(t, "cook", w.p.current)
	})

	t.Run("player saved on cook can turn in", func(t *testing.T) {
		require.True(t, holdsAll(newQuestWalker(q).at("cook").p, turnIn))
	})

	t.Run("each hint names its step's command", func(t *testing.T) {
		want := map[string]string{
			"start": "type track pronghorn",
			"track": "type attack pronghorn",
			"hunt":  "type loot corpse",
			"meat":  "type sleep",
			"rest":  "type craft grilled meat",
			"cook":  "type talk delk", // R20 (#375): talk replaces ask
		}
		for _, s := range q.Steps {
			if s.Id == "end" {
				continue
			}
			wildOneTypedCommand(t, s.Hint)
			require.Contains(t, strings.ToLower(s.Hint), want[s.Id], "step %s", s.Id)
		}
		require.Contains(t, q.Steps[0].Hint, darkAside, "the hunt is a kill quest in rooms that go dark")
	})
}

func TestQuest42_DelkMatchesEachStep(t *testing.T) {
	q := loadShippedQuest(t, "42-the_hunt.yaml", 42)
	df := wildLoadDialogue(t, "9137.yaml")

	want := map[string]string{
		"start": "type track pronghorn",
		"track": "type attack pronghorn",
		"hunt":  "type loot corpse",
		"meat":  "type sleep",
		"rest":  "type craft grilled meat",
		"cook":  "type ask delk report",
	}
	for _, step := range []string{"start", "track", "hunt", "meat", "rest", "cook"} {
		p := newQuestWalker(q).at(step).p
		p.quests["41-end"] = true
		hints := wildNewTalker(p).greet(df)
		wildOneTypedCommand(t, hints)
		require.Contains(t, strings.ToLower(hints), want[step], "Delk's greeting on step %s", step)
	}

	t.Run("mid-quest words get a quest answer, not the catch-all", func(t *testing.T) {
		for _, step := range []string{"start", "track", "hunt", "meat", "rest"} {
			for _, topic := range []string{"pronghorn", "track", "hunt", "meat", "corpse", "salt",
				"sleep", "rest", "cook", "fire", "grilled meat", "help", "how", "quest", "task"} {
				p := newQuestWalker(q).at(step).p
				p.quests["41-end"] = true
				tk := wildNewTalker(p)
				tk.carried[wildSaltPouch] = true
				_, _, node := tk.ask(df, topic)
				require.True(t, node, "ask delk %s on step %s fell through to a pattern", topic, step)
			}
		}
	})

	t.Run("the salt is handed out once", func(t *testing.T) {
		p := newQuestWalker(q).at("hunt").p
		p.quests["41-end"] = true
		tk := wildNewTalker(p)
		_, _, node := tk.ask(df, "salt")
		require.True(t, node)
		require.Equal(t, []int{wildSaltPouch}, tk.given, "a player with no salt gets a pouch")

		text, _, node := tk.ask(df, "salt")
		require.True(t, node, "a player who has salt still gets an answer")
		require.NotEmpty(t, text)
		require.Equal(t, []int{wildSaltPouch}, tk.given, "a player who carries salt gets no second pouch")
	})
}

func TestQuest43BreakThePack_OneCommand(t *testing.T) {
	q := loadShippedQuest(t, "43-break_the_pack.yaml", 43)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)

	start := q.Steps[0]
	wildOneTypedCommand(t, start.Hint)
	require.Contains(t, strings.ToLower(start.Hint), "type attack alpha")
	require.Contains(t, start.Hint, darkAside)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: 9140, RoomId: 5298})
	require.Equal(t, "start", w.p.current, "a hound is not the alpha")
	w.notify("mob_death", EventDetails{MobId: wildAlpha, RoomId: 5298})
	require.Equal(t, "end", w.p.current)

	df := wildLoadDialogue(t, "9137.yaml")
	p := newQuestWalker(q).at("start").p
	p.quests["41-end"], p.quests["42-end"] = true, true
	hints := wildNewTalker(p).greet(df)
	wildOneTypedCommand(t, hints)
	require.Contains(t, strings.ToLower(hints), "type attack alpha")
}

func TestQuest56RunningGame_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "56-running_game.yaml", 56)
	require.Equal(t, []string{"start", "hunt", "end"}, newLinearPlayer(q).steps)

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "forage", RoomId: wildOpenSteppe})
		require.Equal(t, "start", w.p.current, "forage before the kill must not finish the run")
		w.notify("mob_death", EventDetails{MobId: wildPronghorn, RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current)
		w.notify("command", EventDetails{Command: "forage", RoomId: wildDelkCamp})
		require.Equal(t, "hunt", w.p.current, "the forage that finishes it is on the Open Steppe")
		w.notify("command", EventDetails{Command: "forage", RoomId: wildOpenSteppe})
		require.Equal(t, "end", w.p.current)
	})

	// S5: the hint says what actually completes the step. The forage trigger
	// is room 5284 (the Open Steppe), and the run ends on it with no visit to
	// Delk, so no hint may send the player to "the camp" or back to Delk.
	t.Run("hints say what completes each step", func(t *testing.T) {
		start, hunt := q.Steps[0], q.Steps[1]
		wildOneTypedCommand(t, start.Hint)
		wildOneTypedCommand(t, hunt.Hint)
		require.Contains(t, strings.ToLower(start.Hint), "type attack pronghorn")
		require.Contains(t, start.Hint, darkAside)
		require.Contains(t, strings.ToLower(hunt.Hint), "type forage")
		require.Contains(t, hunt.Hint, "Open Steppe")
		require.NotContains(t, strings.ToLower(hunt.Hint), "camp", "the forage is not at the camp")
		require.NotContains(t, hunt.Hint, "Delk", "the run completes on the forage, not at Delk")
	})

	t.Run("Delk greets each step", func(t *testing.T) {
		df := wildLoadDialogue(t, "9137.yaml")
		for step, want := range map[string]string{"start": "type attack pronghorn", "hunt": "type forage"} {
			p := newQuestWalker(q).at(step).p
			p.quests["41-end"], p.quests["42-end"], p.quests["43-end"] = true, true, true
			hints := wildNewTalker(p).greet(df)
			wildOneTypedCommand(t, hints)
			require.Contains(t, strings.ToLower(hints), want, "Delk's greeting on 56 step %s", step)
		}
	})
}

func TestQuest41_StartCarriesDarkAside(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	require.Contains(t, q.Steps[0].Hint, darkAside)
}

func TestQuest41_TarnAnswersQuestWords(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	df := wildLoadDialogue(t, "9136.yaml")
	for _, step := range []string{"start", "track"} {
		for _, topic := range []string{"hare", "track", "forage", "help", "how", "quest", "task"} {
			p := newQuestWalker(q).at(step).p
			p.quests["30-end"] = true
			_, _, node := wildNewTalker(p).ask(df, topic)
			require.True(t, node, "ask tarn %s on step %s fell through to a pattern", topic, step)
		}
	}
}
