package questengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The Wilderness spoke of Pothole Coulee (Scout Tarn 9136, Hunter Delk 9137):
// quests 41, 42, 43 and 56 under the owner's one-command-per-step ruling
// (2026-10-17). The walks use the controller's helpers in
// newbie_one_command_steps_test.go; the dialogue checks run the shipped topics
// through OpenerFor, Eligible and Apply (checkTopicSteps).

const (
	wildPronghorn  = 9139
	wildAlpha      = 9143
	wildRawMeat    = 40014
	wildSaltPouch  = 40017
	wildDelkCamp   = 5290 // Hunter's Hollow, the cooking_fire station
	wildOpenSteppe = 5284
	wildThornbrake = 5289
)

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
	f := "42-the_hunt.yaml"
	pre := []string{"41-end"}
	carriesSalt := func(p *linearPlayer) { p.items[wildSaltPouch] = true }
	checkTopicStepsWith(t, "delk", "9137.yaml", carriesSalt, []topicStep{
		{f, 42, "start", "type track pronghorn", pre, "hunt_progress_start", false},
		{f, 42, "track", "type attack pronghorn", pre, "hunt_progress_track", false},
		{f, 42, "hunt", "type loot corpse", pre, "hunt_progress_hunt", false},
		{f, 42, "meat", "type sleep", pre, "hunt_progress_meat", false},
		{f, 42, "rest", "type craft grilled meat", pre, "hunt_progress_rest", false},
		{f, 42, "cook", "", pre, "hunt_turnin", true},
	})

	t.Run("the salt is handed out once", func(t *testing.T) {
		n := newNpcTalker(loadCouleeTopics(t, "9137.yaml"), playerOn(q, "hunt", "41-end"))
		checkHandout(t, n, "salt_recovery", "salt_have", wildSaltPouch)
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

	checkTopicSteps(t, "delk", "9137.yaml", []topicStep{
		{"43-break_the_pack.yaml", 43, "start", "type attack alpha", []string{"41-end", "42-end"}, "pack_progress", false},
	})
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
		f := "56-running_game.yaml"
		pre := []string{"41-end", "42-end", "43-end"}
		checkTopicSteps(t, "delk", "9137.yaml", []topicStep{
			{f, 56, "start", "type attack pronghorn", pre, "game_progress_start", false},
			{f, 56, "hunt", "type forage", pre, "game_progress_hunt", false},
		})
	})
}

func TestQuest41_StartCarriesDarkAside(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	require.Contains(t, q.Steps[0].Hint, darkAside)
}

func TestQuest41_TarnAnswersQuestWords(t *testing.T) {
	f := "41-first_sign.yaml"
	pre := []string{"30-end"}
	checkTopicSteps(t, "tarn", "9136.yaml", []topicStep{
		{f, 41, "start", "type track hare", pre, "sign_progress_start", false},
		{f, 41, "track", "type forage", pre, "sign_progress_track", false},
		{f, 41, "forage", "", pre, "sign_turnin", true},
	})
}
