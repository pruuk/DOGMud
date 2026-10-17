package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml2 "gopkg.in/yaml.v2"
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

func foldingLoadOrrin(t *testing.T) *dialogue.DialogueFile {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", "9145.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml2.Unmarshal(data, &df), "9145.yaml must parse with the loader's yaml.v2, or Orrin is mute")
	return &df
}

func foldingState(p *linearPlayer) *dialogue.PlayerState {
	return &dialogue.PlayerState{
		HasQuest:  p.HasQuest,
		HasItem:   func(int) bool { return false },
		GiveQuest: func(string) {},
		GiveItem:  func(int) bool { return true },
	}
}

var foldingOrrinInstance = 70000

// foldingAsk runs `ask orrin <topic>` the way usercommands/ask.go does: the
// tree first, then the patterns. It reports the text, the hints and whether
// the catch-all answered.
func foldingAsk(t *testing.T, df *dialogue.DialogueFile, p *linearPlayer, topic string) (string, string, bool) {
	t.Helper()
	foldingOrrinInstance++
	ps := foldingState(p)
	if text, hints, _, ok := dialogue.TreeAdvance(df, foldingOrrinInstance, 1, topic, ps); ok {
		return text, hints, false
	}
	text, _, ok, fallback := dialogue.MatchWithFallbackInfo(df, foldingOrrinInstance, topic, ps)
	require.True(t, ok, "Orrin said nothing to %q", topic)
	return text, "", fallback
}

func foldingGreet(t *testing.T, df *dialogue.DialogueFile, p *linearPlayer) (string, string) {
	t.Helper()
	foldingOrrinInstance++
	text, hints, ok := dialogue.Greet(df, foldingOrrinInstance, 1, foldingState(p))
	require.True(t, ok)
	return text, hints
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

// Orrin answers mid-quest questions about the words his hints use instead of
// falling through to "Say it plainly.", and his greeting names each step's
// one command.
func TestOrrinMidQuestAnswers(t *testing.T) {
	df := foldingLoadOrrin(t)
	q45 := loadShippedQuest(t, "45-the_unquiet_grove.yaml", 45)

	for _, step := range []string{"start", "cast"} {
		p := newLinearPlayer(q45)
		p.current = step
		p.fullMockPlayer.quests["44-end"] = true
		for _, topic := range []string{"wisp", "revenant", "cast", "spike", "grove", "help", "how"} {
			text, _, fallback := foldingAsk(t, df, p, topic)
			require.False(t, fallback, "45-%s: ask orrin %s fell through to the catch-all: %q", step, topic, text)
		}
	}

	greetWants := map[string]string{
		"start": "type cast conviction-spike revenant",
		"cast":  "type cast conviction-spike",
		"kill":  "type ask orrin done",
	}
	for step, want := range greetWants {
		p := newLinearPlayer(q45)
		p.current = step
		p.fullMockPlayer.quests["44-end"] = true
		_, hints := foldingGreet(t, df, p)
		require.Contains(t, strings.ToLower(hints), want, "Orrin's greeting on 45-%s", step)
		require.Equal(t, 1, len(typedCommand.FindAllString(strings.ReplaceAll(hints, darkAside, ""), -1)),
			"Orrin's greeting on 45-%s names one command: %q", step, hints)
	}

	q46 := loadShippedQuest(t, "46-the_unfolded.yaml", 46)
	p := newLinearPlayer(q46)
	p.current = "start"
	p.fullMockPlayer.quests["44-end"] = true
	p.fullMockPlayer.quests["45-end"] = true
	for _, topic := range []string{"unfolded", "scabland", "thinnest", "way", "help"} {
		text, _, fallback := foldingAsk(t, df, p, topic)
		require.False(t, fallback, "46-start: ask orrin %s fell through to the catch-all: %q", topic, text)
	}
	_, hints := foldingGreet(t, df, p)
	require.Contains(t, hints, "west, up, north, west, north and east to the Thinnest Place")

	q57 := loadShippedQuest(t, "57-tending_the_grove.yaml", 57)
	p = newLinearPlayer(q57)
	p.current = "start"
	p.fullMockPlayer.quests["44-end"] = true
	p.fullMockPlayer.quests["45-end"] = true
	p.fullMockPlayer.quests["46-end"] = true
	for _, topic := range []string{"wisp", "grove", "tend", "help"} {
		text, _, fallback := foldingAsk(t, df, p, topic)
		require.False(t, fallback, "57-start: ask orrin %s fell through to the catch-all: %q", topic, text)
	}
	_, hints = foldingGreet(t, df, p)
	require.Contains(t, strings.ToLower(hints), "type cast conviction-spike wisp")
}
