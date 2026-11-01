package questengine

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The tracking line of Pothole Coulee (#375 phase 7): Crier Toke offers First
// Sign (41) and Scout Tarn (9136) takes it; Tarn offers The Hunt (42) with a
// salt pouch and Hunter Delk (9137) takes it; Delk offers Break the Pack (43),
// then Report to Toke (Tracking) (93) and the repeatable Running Game (56).
// One command per step, every quest ends at a hand-in.

const (
	wildPronghorn = 9139
	wildAlpha     = 9143
	wildRawMeat   = 40014
	wildSaltPouch = 40017
	wildDelk      = 9137

	wildGrilledMeat  = 30022 // recipes/cooking/grilled-meat.yaml
	wildTrailRations = 30021 // recipes/cooking/trail-rations.yaml, same fire and materials

	wildTarn       = 9136
	wildDelkCamp   = 5290 // Hunter's Hollow, the cooking_fire station
	wildOpenSteppe = 5284
	wildSteppeEdge = 5283
	wildThornbrake = 5289
)

func wildOneTypedCommand(t *testing.T, hint string) {
	t.Helper()
	n := len(typedCommand.FindAllString(strings.ReplaceAll(hint, darkAside, ""), -1))
	require.Equal(t, 1, n, "a hint names exactly one command: %q", hint)
}

func TestQuest42TheHunt_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "42-the_hunt.yaml", 42)
	require.Equal(t, []string{"start", "met", "track", "hunt", "meat", "rest", "cook", "eaten", "end"}, newLinearPlayer(q).steps,
		"talk, track, kill, take the meat, sleep, cook, eat, report; the old ids keep their places")
	turnIn := turnInRequires(t, "9137.yaml", "hunt_turnin")
	require.Equal(t, []string{"42-eaten"}, turnIn)
	heard := EventDetails{MobId: wildDelk, Topic: "hunt_arrive", RoomId: wildDelkCamp}

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "track", RoomId: wildThornbrake})
		require.Equal(t, "start", w.p.current, "tracking before Delk has spoken must not skip the talk")
		w.notify("topic_heard", heard)
		require.Equal(t, "met", w.p.current)

		w.notify("mob_death", EventDetails{MobId: wildPronghorn, RoomId: wildThornbrake})
		w.notify("item_gain", EventDetails{ItemId: wildRawMeat, RoomId: wildThornbrake})
		w.notify("command", EventDetails{Command: "sleep", RoomId: wildDelkCamp})
		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp, ItemId: wildGrilledMeat})
		w.notify("command", EventDetails{Command: "eat", RoomId: wildDelkCamp})
		require.Equal(t, "met", w.p.current, "only the track starts the hunt")

		w.notify("command", EventDetails{Command: "track", RoomId: wildThornbrake})
		require.Equal(t, "track", w.p.current)
		w.notify("mob_death", EventDetails{MobId: wildAlpha, RoomId: wildThornbrake})
		require.Equal(t, "track", w.p.current, "only a pronghorn is the kill")
		w.notify("mob_death", EventDetails{MobId: wildPronghorn, RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current)

		w.notify("item_gain", EventDetails{ItemId: wildSaltPouch, RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current, "only the meat is the meat")
		w.notify("item_gain", EventDetails{ItemId: wildRawMeat, RoomId: wildThornbrake})
		require.Equal(t, "meat", w.p.current)

		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp, ItemId: wildGrilledMeat})
		require.Equal(t, "meat", w.p.current, "cooking before the rest must not skip it")
		w.notify("command", EventDetails{Command: "sleep", RoomId: wildThornbrake})
		require.Equal(t, "rest", w.p.current)

		w.notify("command", EventDetails{Command: "eat", RoomId: wildThornbrake, ItemId: wildTrailRations})
		require.Equal(t, "rest", w.p.current, "eating before the cook must not skip it")

		w.notify("command", EventDetails{Command: "craft", RoomId: wildThornbrake, ItemId: wildGrilledMeat})
		require.Equal(t, "rest", w.p.current, "a craft away from Delk's fire is not the cook")
		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp, ItemId: wildTrailRations})
		require.Equal(t, "rest", w.p.current, "trail rations at Delk's fire are not the grilled meat")
		w.notify("command", EventDetails{Command: "craft", RoomId: wildDelkCamp, ItemId: wildGrilledMeat})
		require.Equal(t, "cook", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in waits for the meal to be eaten")

		w.notify("command", EventDetails{Command: "eat", RoomId: wildDelkCamp, ItemId: wildGrilledMeat})
		require.Equal(t, "eaten", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("each hint names its step's command", func(t *testing.T) {
		want := map[string]string{
			"start": "type talk delk",
			"met":   "type track pronghorn",
			"track": "type attack pronghorn",
			"hunt":  "type loot corpse",
			"meat":  "type sleep",
			"rest":  "type craft grilled meat",
			"cook":  "type eat grilled meat",
			"eaten": "type talk delk",
		}
		for _, s := range q.Steps {
			if s.Id == "end" {
				continue
			}
			wildOneTypedCommand(t, s.Hint)
			require.Contains(t, strings.ToLower(s.Hint), want[s.Id], "step %s", s.Id)
		}
		require.Contains(t, q.Steps[1].Hint, darkAside, "the hunt is a kill quest in rooms that go dark")
	})

	// A failed cook and a batch of trail rations each use up the meat and the
	// salt (recipes cooking/grilled-meat and trail-rations), and the step stays
	// open. Both ways forward must be named: a new pronghorn, and Delk's salt.
	t.Run("a burned meal or trail rations leave a way forward", func(t *testing.T) {
		delk := loadCouleeTopics(t, "9137.yaml")
		n := newNpcTalker(delk, playerOn(q, "rest", "30-end", "41-end"))
		require.NotNil(t, n.eligible("salt_recovery"), "Delk replaces the salt the craft used up")

		openerText, _ := n.opener()
		for name, line := range map[string]string{
			"rest hint":        questHintFor(t, q, "rest"),
			"rest description": q.Steps[5].Description,
			"Delk opener":      openerText,
			"hunt_progress":    delk.FindTopic("hunt_progress_rest").Text,
		} {
			s := squash(line)
			require.Contains(t, s, "pronghorn", name)
			require.Contains(t, s, "salt", name)
			require.Contains(t, s, "grilled meat", name)
			require.NotContains(t, s, "the meat is gone", name)
			require.NotContains(t, s, "the meat is lost", name)
		}
		require.NotContains(t, delk.FindTopic("salt_recovery").Text, "Last pinch",
			"a second burn sends the player back for more")
	})

	t.Run("Tarn offers it with the salt and Delk no longer does", func(t *testing.T) {
		checkOfferMoved(t, "9136.yaml", "hunt_offer", q, "41-end", "9137.yaml")
		require.Equal(t, wildSaltPouch, loadCouleeTopics(t, "9136.yaml").FindTopic("hunt_offer").GivesItem)
	})

	t.Run("Delk's talk step", func(t *testing.T) {
		checkArriveTopic(t, "delk", "9137.yaml", q, wildDelk, "hunt_arrive", []string{"30-end", "41-end"})
	})
}

func TestQuest42_DelkMatchesEachStep(t *testing.T) {
	q := loadShippedQuest(t, "42-the_hunt.yaml", 42)
	f := "42-the_hunt.yaml"
	pre := []string{"30-end", "41-end"}
	carriesSalt := func(p *linearPlayer) { p.items[wildSaltPouch] = true }
	checkTopicStepsWith(t, "delk", "9137.yaml", carriesSalt, []topicStep{
		{f, 42, "met", "type track pronghorn", pre, "hunt_progress_start", false},
		{f, 42, "track", "type attack pronghorn", pre, "hunt_progress_track", false},
		{f, 42, "hunt", "type loot corpse", pre, "hunt_progress_hunt", false},
		{f, 42, "meat", "type sleep", pre, "hunt_progress_meat", false},
		{f, 42, "rest", "type craft grilled meat", pre, "hunt_progress_rest", false},
		{f, 42, "cook", "type eat grilled meat", pre, "hunt_progress_cook", false},
		{f, 42, "eaten", "", pre, "hunt_turnin", true},
	})

	t.Run("the salt is handed out once", func(t *testing.T) {
		n := newNpcTalker(loadCouleeTopics(t, "9137.yaml"), playerOn(q, "hunt", "41-end"))
		checkHandout(t, n, "salt_recovery", "salt_have", wildSaltPouch)
	})

	t.Run("Delk's field help teaches eating", func(t *testing.T) {
		tp := loadCouleeTopics(t, "9137.yaml").FindTopic("hunt_help")
		require.NotNil(t, tp)
		require.Contains(t, squash(tp.Text), "type eat grilled meat")
	})
}

func TestQuest43BreakThePack_KillThenReport(t *testing.T) {
	q := loadShippedQuest(t, "43-break_the_pack.yaml", 43)
	require.Equal(t, []string{"start", "slain", "end"}, newLinearPlayer(q).steps)
	start := q.Steps[0]
	wildOneTypedCommand(t, start.Hint)
	require.Contains(t, strings.ToLower(start.Hint), "type attack alpha")
	require.Contains(t, start.Hint, darkAside)
	turnIn := turnInRequires(t, "9137.yaml", "pack_in")
	require.Equal(t, []string{"43-slain"}, turnIn)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: 9140, RoomId: 5298})
	require.Equal(t, "start", w.p.current, "a hound is not the alpha")
	w.notify("mob_death", EventDetails{MobId: wildAlpha, RoomId: 5298})
	require.Equal(t, "slain", w.p.current)
	require.False(t, holdsAll(w.p, []string{"43-end"}), "the kill no longer ends the quest")

	pre := []string{"30-end", "41-end", "42-end"}
	checkTopicSteps(t, "delk", "9137.yaml", []topicStep{
		{"43-break_the_pack.yaml", 43, "start", "type attack alpha", pre, "pack_progress", false},
		{"43-break_the_pack.yaml", 43, "slain", "", pre, "pack_in", true},
	})
}

// The jerkin (quest 43's recipe reward) is sewable by a fresh player once
// known (spec "Skill coverage" ruling), and learned only from that reward,
// never discovered at random (owner ruling 2026-10-30, learn_only).
func TestScabHideJerkin_SkillMinimumOneAndLearnOnly(t *testing.T) {
	raw := string(repoFile(t, "_datafiles", "world", "dogmud", "recipes", "tailoring", "scab-hide-jerkin.yaml"))
	require.Regexp(t, regexp.MustCompile(`(?m)^skill_minimum: 1\r?$`), raw)
	require.Regexp(t, regexp.MustCompile(`(?m)^learn_only: true\r?$`), raw)
	q := loadShippedQuest(t, "43-break_the_pack.yaml", 43)
	require.Contains(t, q.Rewards.RecipeInfo, "scab-hide-jerkin", "quest 43's reward is the only way to learn it")
}

func TestQuest56RunningGame_EndsAtDelk(t *testing.T) {
	q := loadShippedQuest(t, "56-running_game.yaml", 56)
	require.Equal(t, []string{"start", "hunt", "foraged", "end"}, newLinearPlayer(q).steps)
	turnIn := turnInRequires(t, "9137.yaml", "game_loop_in")
	require.Equal(t, []string{"56-foraged"}, turnIn)

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "forage", RoomId: wildOpenSteppe})
		require.Equal(t, "start", w.p.current, "forage before the kill must not count")
		w.notify("mob_death", EventDetails{MobId: wildPronghorn, RoomId: wildThornbrake})
		require.Equal(t, "hunt", w.p.current)
		w.notify("command", EventDetails{Command: "forage", RoomId: wildDelkCamp})
		require.Equal(t, "hunt", w.p.current, "the forage that counts is on the Open Steppe")
		w.notify("command", EventDetails{Command: "forage", RoomId: wildOpenSteppe})
		require.Equal(t, "foraged", w.p.current)
		require.False(t, holdsAll(w.p, []string{"56-end"}), "the forage no longer ends the run")
	})

	t.Run("hints say what completes each step", func(t *testing.T) {
		start, hunt, foraged := q.Steps[0], q.Steps[1], q.Steps[2]
		wildOneTypedCommand(t, start.Hint)
		wildOneTypedCommand(t, hunt.Hint)
		wildOneTypedCommand(t, foraged.Hint)
		require.Contains(t, strings.ToLower(start.Hint), "type attack pronghorn")
		require.Contains(t, start.Hint, darkAside)
		require.Contains(t, strings.ToLower(hunt.Hint), "type forage")
		require.Contains(t, hunt.Hint, "Open Steppe")
		require.Contains(t, strings.ToLower(foraged.Hint), "type talk delk")
	})

	t.Run("Delk greets each step", func(t *testing.T) {
		f := "56-running_game.yaml"
		pre := []string{"30-end", "41-end", "42-end", "43-end"}
		checkTopicSteps(t, "delk", "9137.yaml", []topicStep{
			{f, 56, "start", "type attack pronghorn", pre, "game_progress_start", false},
			{f, 56, "hunt", "type forage", pre, "game_progress_hunt", false},
			{f, 56, "foraged", "", pre, "game_loop_in", true},
		})
	})
}

func TestQuest41_TrackStepCarriesDarkAside(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	require.Contains(t, q.Steps[1].Hint, darkAside)
}

func TestQuest41_TarnAnswersQuestWords(t *testing.T) {
	q := loadShippedQuest(t, "41-first_sign.yaml", 41)
	f := "41-first_sign.yaml"
	pre := []string{"30-end", "31-end"}
	checkTopicSteps(t, "tarn", "9136.yaml", []topicStep{
		{f, 41, "met", "type track hare", pre, "sign_progress_start", false},
		{f, 41, "track", "type forage", pre, "sign_progress_track", false},
		{f, 41, "forage", "", pre, "sign_turnin", true},
	})
	checkArriveTopic(t, "tarn", "9136.yaml", q, wildTarn, "sign_arrive", pre)
}
