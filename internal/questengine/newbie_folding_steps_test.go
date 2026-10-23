package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// The Folding spoke's grove quests (Keeper Orrin, Pothole Coulee) under the
// owner ruling of 2026-10-17: one command per step, steps in order, true
// directions, and a giver who answers the words his own hints invite.
// Identifiers carry a `folding` prefix: sibling newbie test files share this
// package.

const (
	foldingOrrinRoom     = 5309 // Whispering Grove
	foldingGroveEdge     = 5308
	foldingVeilHollow    = 5311
	foldingThinningWood  = 5312
	foldingGroveHeart    = 5313
	foldingThinnestPlace = 5320
	foldingDriftWisp     = 9147
	foldingGroveRevenant = 9148
	foldingTheUnfolded   = 9152
)

// foldingRoom is a coulee room's exits and spawns, read from the shipped YAML.
type foldingRoom struct {
	Title string `yaml:"title"`
	Exits map[string]struct {
		RoomId int `yaml:"roomid"`
	} `yaml:"exits"`
	SpawnInfo []struct {
		MobId int `yaml:"mobid"`
	} `yaml:"spawninfo"`
}

func foldingLoadRoom(t *testing.T, id int) foldingRoom {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "rooms", "pothole_coulee",
		strconv.Itoa(id)+".yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "room %d must exist", id)
	var r foldingRoom
	require.NoError(t, yaml.Unmarshal(data, &r))
	return r
}

// foldingWalk follows exits from a room and returns where the player ends up.
func foldingWalk(t *testing.T, from int, dirs ...string) int {
	t.Helper()
	at := from
	for _, d := range dirs {
		r := foldingLoadRoom(t, at)
		ex, ok := r.Exits[d]
		require.True(t, ok, "room %d (%s) has no %s exit", at, r.Title, d)
		at = ex.RoomId
	}
	return at
}

func foldingSpawns(t *testing.T, room int, mob int) bool {
	t.Helper()
	for _, s := range foldingLoadRoom(t, room).SpawnInfo {
		if s.MobId == mob {
			return true
		}
	}
	return false
}

func foldingHint(q *QuestDef, id string) string {
	for _, s := range q.Steps {
		if s.Id == id {
			return s.Hint
		}
	}
	return ""
}

// The Unquiet Grove: cast in the grove fight first, then finish the creature.
// The quest asks for a spell "in the thick of the grove where the foes are",
// with "a fight on you"; casting at the creature and then bringing it down is
// one fight. Killing first and casting after needs a second fight.
func TestQuest45UnquietGrove_CastThenKill(t *testing.T) {
	q := loadShippedQuest(t, "45-the_unquiet_grove.yaml", 45)
	require.Equal(t, []string{"start", "cast", "kill", "end"}, newLinearPlayer(q).steps,
		"cast comes before kill; both ids survive so a saved player resumes")
	turnIn := turnInRequires(t, "9145.yaml", "grove_turnin")
	require.Equal(t, []string{"45-kill"}, turnIn, "the turn-in gates on the last step")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("mob_death", EventDetails{MobId: foldingGroveRevenant, RoomId: foldingVeilHollow})
		require.Equal(t, "start", w.p.current, "a kill before the cast must not count")

		w.notify("command", EventDetails{Command: "cast", Noun: "chrysalis-glow", RoomId: foldingVeilHollow})
		require.Equal(t, "start", w.p.current, "lighting the grove is not the combat cast")

		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingOrrinRoom})
		require.Equal(t, "start", w.p.current, "the cast counts only in the grove's fight rooms")

		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingVeilHollow})
		require.Equal(t, "cast", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the kill")

		w.notify("mob_death", EventDetails{MobId: foldingGroveRevenant, RoomId: foldingVeilHollow})
		require.Equal(t, "kill", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the kill")
	})

	t.Run("a wisp in the Thinning Wood serves too", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingThinningWood})
		require.Equal(t, "cast", w.p.current)
		w.notify("mob_death", EventDetails{MobId: foldingDriftWisp, RoomId: foldingThinningWood})
		require.Equal(t, "kill", w.p.current)
	})

	t.Run("player saved on cast resumes with the kill", func(t *testing.T) {
		w := newQuestWalker(q).at("cast")
		require.False(t, holdsAll(w.p, turnIn))
		w.notify("mob_death", EventDetails{MobId: foldingDriftWisp, RoomId: foldingGroveHeart})
		require.Equal(t, "kill", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on kill can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("kill")
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("directions are true", func(t *testing.T) {
		require.Contains(t, foldingHint(q, "start"), "Go west to the Veil Hollow")
		require.Equal(t, foldingVeilHollow, foldingWalk(t, foldingOrrinRoom, "west"))
		require.True(t, foldingSpawns(t, foldingVeilHollow, foldingGroveRevenant), "the start hint names the Veil Hollow's revenant")
		require.Contains(t, foldingHint(q, "start"), darkAside)
	})
}

// The Unfolded: one boss kill, with a route that walks.
func TestQuest46Unfolded_RouteAndKill(t *testing.T) {
	q := loadShippedQuest(t, "46-the_unfolded.yaml", 46)
	require.Equal(t, []string{"start", "end"}, newLinearPlayer(q).steps)

	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: foldingGroveRevenant, RoomId: foldingGroveHeart})
	require.Equal(t, "start", w.p.current)
	w.notify("mob_death", EventDetails{MobId: foldingTheUnfolded, RoomId: foldingThinnestPlace})
	require.Equal(t, "end", w.p.current)

	hint := foldingHint(q, "start")
	require.Contains(t, hint, darkAside)
	require.Contains(t, hint, "west, north and north to the Grove Heart")
	require.Equal(t, foldingGroveHeart, foldingWalk(t, foldingOrrinRoom, "west", "north", "north"))
	require.Contains(t, hint, "west, up, north, west, north and east to the Thinnest Place")
	require.Equal(t, foldingThinnestPlace, foldingWalk(t, foldingGroveHeart, "west", "up", "north", "west", "north", "east"))
	require.True(t, foldingSpawns(t, foldingThinnestPlace, foldingTheUnfolded))
}

// Tending the Grove ends on the kill, so nothing may send the player back.
func TestQuest57TendingTheGrove_EndsOnTheKill(t *testing.T) {
	q := loadShippedQuest(t, "57-tending_the_grove.yaml", 57)
	w := newQuestWalker(q).at("start")
	w.notify("mob_death", EventDetails{MobId: foldingGroveRevenant, RoomId: foldingVeilHollow})
	require.Equal(t, "start", w.p.current)
	w.notify("mob_death", EventDetails{MobId: foldingDriftWisp, RoomId: foldingGroveEdge})
	require.Equal(t, "end", w.p.current)

	var texts []string
	for _, s := range q.Steps {
		texts = append(texts, s.Hint)
	}
	for _, tr := range q.Triggers {
		for _, a := range tr.Actions {
			texts = append(texts, a.SendText)
		}
	}
	for _, s := range texts {
		low := strings.ToLower(s)
		for _, bad := range []string{"come back", "go back", "tell orrin", "return to orrin"} {
			require.NotContains(t, low, bad, "quest 57 completes on the kill; this text sends the player back: %q", s)
		}
	}

	require.Contains(t, foldingHint(q, "start"), "Go south to the Grove Edge")
	require.Equal(t, foldingGroveEdge, foldingWalk(t, foldingOrrinRoom, "south"))
	require.True(t, foldingSpawns(t, foldingGroveEdge, foldingDriftWisp))
}

// Orrin's opener names each grove step's one command, and a quest-aware topic
// on his menu answers it (the old `ask orrin wisp/revenant/cast` words); the
// kill step's hand-in completes The Unquiet Grove.
func TestOrrinMidQuestAnswers(t *testing.T) {
	f45 := "45-the_unquiet_grove.yaml"
	checkTopicSteps(t, "orrin", "9145.yaml", []topicStep{
		{f45, 45, "start", "type cast spike revenant", []string{"44-end"}, "grove_mid_cast", false},
		{f45, 45, "cast", "type cast spike", []string{"44-end"}, "grove_mid_kill", false},
		{f45, 45, "kill", "", []string{"44-end"}, "grove_turnin", true},
		{"46-the_unfolded.yaml", 46, "start", "west, up, north, west, north and east to the thinnest place",
			[]string{"44-end", "45-end"}, "scab_mid", false},
		{"57-tending_the_grove.yaml", 57, "start", "type cast spike wisp",
			[]string{"44-end", "45-end", "46-end"}, "grove_loop_mid", false},
	})
}
