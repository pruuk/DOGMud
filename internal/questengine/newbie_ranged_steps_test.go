package questengine

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// The ranged spoke after the owner ruling (2026-10-17): Across the Canyon
// (51), The Overlook (52) and Range Practice (59) ask for one command per
// step, in order, and their givers (Marksman Iden 9161, Scout Bryn 9162)
// greet each step with that step's command, hand ammunition and weapons out
// only to a player who does not already carry one, and answer the obvious
// mid-quest questions with the step instead of the catch-all.

const (
	canyonNarrows  = 5354 // Echoing Narrows: the shot is taken FROM here
	soundingFloor  = 5356 // The Sounding Floor: where the scout stands
	canyonScout    = 9165
	bowmasterSkell = 9169
	pouchOfShot    = 30064
	caseOfBolts    = 30063
	slingItem      = 10038
)

// squash lowercases and collapses whitespace, so a folded YAML line compares
// the way the player reads it.
func squash(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func readQuestFile(t *testing.T, file string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "quests", file))
	require.NoError(t, err)
	return string(raw)
}

// playerOn builds a linear player for quest q at step, holding the given
// tokens from other quests (the prerequisites a real character would carry).
func playerOn(q *QuestDef, step string, other ...string) *linearPlayer {
	p := newLinearPlayer(q)
	p.current = step
	for _, tok := range other {
		p.fullMockPlayer.quests[tok] = true
	}
	return p
}

// questHintFor returns the step's own hint, squashed.
func questHintFor(t *testing.T, q *QuestDef, step string) string {
	t.Helper()
	for _, s := range q.Steps {
		if s.Id == step {
			return squash(s.Hint)
		}
	}
	t.Fatalf("quest %d has no step %q", q.QuestId, step)
	return ""
}

func TestQuest51AcrossTheCanyon_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "51-across_the_canyon.yaml", 51)
	require.Equal(t, []string{"start", "shoot", "kill", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; keep them so a player mid-quest resumes")
	turnIn := turnInRequires(t, "9162.yaml", "canyon_turnin")
	require.Equal(t, []string{"51-kill"}, turnIn, "the turn-in gates on the last step's token")

	require.Contains(t, questHintFor(t, q, "start"), "type fire scout north")
	require.Contains(t, questHintFor(t, q, "shoot"), "type fire scout")
	require.Contains(t, questHintFor(t, q, "kill"), "type talk bryn") // R20 (#375): talk replaces ask

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		// Killing the scout before the cross-room shot is not the lesson.
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: soundingFloor})
		require.Equal(t, "start", w.p.current, "a kill before the cross-room shot must not skip the shot")
		require.False(t, holdsAll(w.p, turnIn))

		// A shot from the wrong room is not the cross-room shot.
		w.notify("command", EventDetails{Command: "fire", RoomId: soundingFloor})
		require.Equal(t, "start", w.p.current, "only a shot from the Echoing Narrows counts")

		w.notify("command", EventDetails{Command: "fire", RoomId: canyonNarrows})
		require.Equal(t, "shoot", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in waits for the kill")

		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
		require.Equal(t, "kill", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the kill")
	})

	// A player saved on start who already killed the scout some other way
	// (walked up and cut it down) is not stuck: the scout respawns on the
	// Sounding Floor, and the start hint asks for the shot from the Narrows.
	t.Run("player saved on start who killed the scout another way can still finish", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: soundingFloor})
		require.Equal(t, "start", w.p.current)
		w.notify("command", EventDetails{Command: "fire", RoomId: canyonNarrows})
		require.Equal(t, "shoot", w.p.current)
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
		require.Equal(t, "kill", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on shoot resumes with the kill", func(t *testing.T) {
		w := newQuestWalker(q).at("shoot")
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
		require.Equal(t, "kill", w.p.current)
	})

	t.Run("player saved on kill can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("kill")
		require.True(t, holdsAll(w.p, turnIn))
	})
}

func TestQuest52TheOverlook_OneCommandStart(t *testing.T) {
	q := loadShippedQuest(t, "52-the_overlook.yaml", 52)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, questHintFor(t, q, "start"), "type fire skell")
	require.Contains(t, q.Steps[0].Hint, darkAside, "the Overlook goes dark at night; the start hint carries the dark aside")
	require.NotContains(t, questHintFor(t, q, "start"), "ammo",
		"ammo advice is not the step's command; it belongs in the description")
	require.Contains(t, squash(q.Steps[0].Description), "ammo")

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
	require.Equal(t, "start", w.p.current, "only Skell's death finishes the Overlook")
	w.notify("mob_death", EventDetails{MobId: bowmasterSkell, RoomId: 5364})
	require.Equal(t, "end", w.p.current)
}

func TestQuest59RangePractice_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "59-range_practice.yaml", 59)
	require.Equal(t, []string{"start", "shoot", "end"}, newLinearPlayer(q).steps)
	require.Contains(t, questHintFor(t, q, "start"), "type fire scout north")
	require.Contains(t, questHintFor(t, q, "shoot"), "type fire scout")

	// S5: the drill finishes itself on the kill, so nothing may send the
	// player back to Bryn to report it.
	for _, s := range q.Steps {
		h := squash(s.Hint + " " + s.Description)
		require.NotContains(t, h, "come back", "step %q tells the player to come back, but the drill ends on the kill", s.Id)
		require.NotContains(t, h, "tell bryn", "step %q", s.Id)
	}
	for _, tr := range q.Triggers {
		for _, a := range tr.Actions {
			l := squash(a.SendText)
			require.NotContains(t, l, "come back", "trigger %s/%s line sends the player back", tr.Event, tr.Command)
			require.NotContains(t, l, "tell bryn", "trigger %s/%s", tr.Event, tr.Command)
		}
	}

	// N13: Bryn is "he" (mobs/pothole_coulee/9162-scout_bryn.yaml).
	require.NotRegexp(t, regexp.MustCompile(`(?i)\b(she|her|hers|herself)\b`), readQuestFile(t, "59-range_practice.yaml"),
		"Bryn is a he")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: soundingFloor})
		require.Equal(t, "start", w.p.current, "a kill before the shot does not finish the drill")
		w.notify("command", EventDetails{Command: "fire", RoomId: canyonNarrows})
		require.Equal(t, "shoot", w.p.current)
		w.notify("mob_death", EventDetails{MobId: canyonScout, RoomId: canyonNarrows})
		require.Equal(t, "end", w.p.current, "the drill completes on the kill, with no report")
	})
}

// Bryn's opener names each mid-quest step's command, and a quest-aware topic
// on his menu answers it (the topics the old `ask bryn done/quest/scout`
// words reached). The kill step's hand-in completes Across the Canyon.
func TestBryn9162_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	f51 := "51-across_the_canyon.yaml"
	checkTopicSteps(t, "bryn", "9162.yaml", []topicStep{
		{f51, 51, "start", "type fire scout north", []string{"50-end"}, "coach_51_start", false},
		{f51, 51, "shoot", "type fire scout", []string{"50-end"}, "coach_51_shoot", false},
		{f51, 51, "kill", "", []string{"50-end"}, "canyon_turnin", true},
		{"52-the_overlook.yaml", 52, "start", "type fire skell", []string{"50-end", "51-end"}, "coach_52_start", false},
		{"59-range_practice.yaml", 59, "start", "type fire scout north", []string{"50-end", "51-end", "52-end"}, "coach_59_start", false},
		{"59-range_practice.yaml", 59, "shoot", "type fire scout", []string{"50-end", "51-end", "52-end"}, "coach_59_shoot", false},
	})
}

// S8: a handout topic gives its item once. A player who already carries one
// sees the reminder instead, and gets nothing.
func TestRangedGivers_HandoutsGiveOnlyWhenTheItemIsMissing(t *testing.T) {
	q50 := loadShippedQuest(t, "50-first_shot.yaml", 50)
	q51 := loadShippedQuest(t, "51-across_the_canyon.yaml", 51)
	q52 := loadShippedQuest(t, "52-the_overlook.yaml", 52)
	iden := loadCouleeTopics(t, "9161.yaml")
	bryn := loadCouleeTopics(t, "9162.yaml")

	cases := []struct {
		name         string
		df           *dialogue.DialogueFile
		p            func() *linearPlayer
		supply, have string
		item         int
	}{
		{"iden shot", iden, func() *linearPlayer { return playerOn(q50, "equip", "30-end") }, "shot_supply", "shot_have", pouchOfShot},
		{"iden sling", iden, func() *linearPlayer { return playerOn(q50, "equip", "30-end") }, "sling_recovery", "sling_have", slingItem},
		{"bryn shot during 51", bryn, func() *linearPlayer { return playerOn(q51, "start", "50-end") }, "canyon_shot_supply", "canyon_shot_have", pouchOfShot},
		{"bryn bolts during 52", bryn, func() *linearPlayer { return playerOn(q52, "start", "50-end", "51-end") }, "ammo_resupply", "ammo_have", caseOfBolts},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkHandout(t, newNpcTalker(c.df, c.p()), c.supply, c.have, c.item)
		})
	}
}
