package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// The casting line (Pothole Coulee quests 44, 45, 46 and the repeatable 57)
// under #375 phase 7: giver to giver. Toke offers First Casting and Grieve
// takes it; Grieve offers The Unquiet Grove and Orrin takes it; Orrin offers
// The Unfolded, takes its report, and keeps Tending the Grove and the bridge
// back to Toke. One command per step, true directions, and a giver who
// answers the words his own hints invite. Identifiers carry a `folding`
// prefix: sibling newbie test files share this package.

const (
	foldingHubSquare     = 5203
	foldingObservatory   = 5303
	foldingStarChamber   = 5305
	foldingOrrinRoom     = 5309
	foldingGroveEdge     = 5308
	foldingVeilHollow    = 5311
	foldingThinningWood  = 5312
	foldingGroveHeart    = 5313
	foldingThinnestPlace = 5320
	foldingGrieve        = 9144
	foldingOrrin         = 9145
	foldingPracticeMote  = 9146
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

// foldingInstallGrove installs the shipped unquiet_grove area (quest 45's
// cast step is scoped to it) over the coulee rooms it names.
func foldingInstallGrove(t *testing.T) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "areas", "unquiet_grove.yaml"))
	require.NoError(t, err)
	var a areas.Area
	require.NoError(t, yaml.Unmarshal(data, &a))
	zones := map[int]string{}
	for _, id := range []int{foldingOrrinRoom, foldingGroveEdge, foldingVeilHollow, foldingThinningWood, foldingGroveHeart, foldingThinnestPlace} {
		zones[id] = "Pothole Coulee"
	}
	t.Cleanup(areas.InstallForTest([]areas.Area{a}, zones))
}

func foldingHint(q *QuestDef, id string) string {
	for _, s := range q.Steps {
		if s.Id == id {
			return s.Hint
		}
	}
	return ""
}

// foldingOffers maps each quest a Coulee dialogue file offers to its topic key.
func foldingOffers(t *testing.T, file string) map[int]string {
	t.Helper()
	out := map[int]string{}
	for _, tp := range loadCouleeTopics(t, file).Topics {
		if tp.Kind == dialogue.KindOffer {
			out[tp.Quest] = tp.Key
		}
	}
	return out
}

// First Casting: Toke sends the player to Grieve, Grieve's lesson opens the
// cast, and the cast is reported back to Grieve.
func TestQuest44FirstCasting_GiverToGiver(t *testing.T) {
	q := loadShippedQuest(t, "44-first_casting.yaml", 44)
	require.Equal(t, []string{"start", "met", "cast", "end"}, newLinearPlayer(q).steps,
		"start and cast keep their ids so a saved player resumes; met is the meeting with Grieve")
	turnIn := turnInRequires(t, "9144.yaml", "cast_turnin")
	require.Equal(t, []string{"44-cast"}, turnIn, "the turn-in gates on the last step")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingStarChamber})
		require.Equal(t, "start", w.p.current, "a cast before Grieve's lesson must not count")
		w.notify("topic_heard", EventDetails{MobId: foldingGrieve, Topic: "cast_lesson"})
		require.Equal(t, "met", w.p.current)
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingObservatory, MobId: foldingPracticeMote})
		require.Equal(t, "met", w.p.current, "the cast counts only in the Star Chamber")
		// The cast line says the spell met the mote, so a cast at nothing, or
		// at another creature, must not satisfy the step.
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingStarChamber})
		require.Equal(t, "met", w.p.current, "a cast at no mote must not count")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingStarChamber, MobId: foldingGrieve})
		require.Equal(t, "met", w.p.current, "a cast at another creature must not count")
		w.notify("command", EventDetails{Command: "cast", Noun: "conviction-spike", RoomId: foldingStarChamber, MobId: foldingPracticeMote})
		require.Equal(t, "cast", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on cast can turn in", func(t *testing.T) {
		require.True(t, holdsAll(newQuestWalker(q).at("cast").p, turnIn))
	})

	// #697: the mote fights back and a second casting can scatter it, so no
	// line may promise it unharmed, and its own description says so.
	t.Run("nothing promises the mote is unharmed", func(t *testing.T) {
		for _, tr := range q.Triggers {
			for _, a := range tr.Actions {
				require.NotContains(t, strings.ToLower(a.SendText), "unharmed")
			}
		}
		mote := strings.ToLower(string(repoFile(t, "_datafiles", "world", "dogmud", "mobs", "pothole_coulee", "9146-practice_mote.yaml")))
		require.NotContains(t, mote, "cannot hurt you")
		require.Contains(t, mote, "scatters it")
	})

	t.Run("directions are true", func(t *testing.T) {
		require.Contains(t, squash(foldingHint(q, "start")), "west, north, north, north, up and up to the observatory hall")
		require.Equal(t, foldingObservatory, foldingWalk(t, foldingHubSquare, "west", "north", "north", "north", "up", "up"))
		require.Contains(t, foldingHint(q, "met"), "Go north into the Star Chamber")
		require.Equal(t, foldingStarChamber, foldingWalk(t, foldingObservatory, "north"))
		require.True(t, foldingSpawns(t, foldingStarChamber, foldingPracticeMote))
	})
}

// Grieve's opener and topics follow each step of First Casting, and Grieve
// teaches where spells come from and what manifestation is.
func TestGrieve9144_TopicsNameEachStep(t *testing.T) {
	f := "44-first_casting.yaml"
	pre := []string{"30-end", "31-end"}
	checkMeetStep(t, "grieve", "9144.yaml", f, 44, "cast_lesson", "type cast spike mote", pre...)
	checkTopicSteps(t, "grieve", "9144.yaml", []topicStep{
		{f, 44, "met", "type cast spike mote", pre, "cast_progress", false},
		{f, 44, "cast", "", pre, "cast_turnin", true},
	})

	grieve := loadCouleeTopics(t, "9144.yaml")
	for key, help := range map[string]string{
		"new_spells":    "type help spells",
		"manifestation": "type help manifestation",
		"conjure":       "type help conjure-fire",
		"charm":         "type help charm",
	} {
		tp := grieve.FindTopic(key)
		require.NotNil(t, tp, "Grieve has no %s topic", key)
		require.Equal(t, dialogue.KindInfo, tp.Kind)
		require.Contains(t, squash(tp.Hints), help, "%s points at its help page", key)
	}
	require.Contains(t, strings.ToLower(grieve.FindTopic("new_spells").Text), "not from a trainer",
		"spells come from casting, not trainers (templates/help/spells.template)")
}
