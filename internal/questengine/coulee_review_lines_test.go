package questengine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// Lines on the casting, lore and archery trails (#375 phase 7) checked against
// the spawn data, the quest triggers and the menu order they describe.

type couleeSpawn struct {
	MobId          int    `yaml:"mobid"`
	Message        string `yaml:"message"`
	NarrateRespawn bool   `yaml:"narraterespawn"`
	RespawnRate    string `yaml:"respawnrate"`
}

type couleeRoom struct {
	RoomId    int           `yaml:"roomid"`
	Title     string        `yaml:"title"`
	SpawnInfo []couleeSpawn `yaml:"spawninfo"`
}

func couleeWorld(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud")
}

// couleeRoomsSpawning walks every shipped room and returns those spawning mob.
func couleeRoomsSpawning(t *testing.T, mob int) []couleeRoom {
	t.Helper()
	var out []couleeRoom
	root := filepath.Join(couleeWorld(t), "rooms")
	require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var r couleeRoom
		if yaml.Unmarshal(data, &r) != nil {
			return nil // not this test's business; the boot door reports it
		}
		for _, s := range r.SpawnInfo {
			if s.MobId == mob {
				out = append(out, r)
				break
			}
		}
		return nil
	}))
	require.NotEmpty(t, out, "no shipped room spawns mob %d", mob)
	return out
}

type couleeTrigger struct {
	Event      string `yaml:"event"`
	Command    string `yaml:"command"`
	Mob        int    `yaml:"mob"`
	Room       int    `yaml:"room"`
	Area       string `yaml:"area"`
	Zone       string `yaml:"zone"`
	Topic      string `yaml:"topic"`
	Conditions struct {
		Has []string `yaml:"has"`
	} `yaml:"conditions"`
	Actions []struct {
		Grant string `yaml:"grant"`
		Actor string `yaml:"actor"`
	} `yaml:"actions"`
}

func couleeTriggers(t *testing.T, file string) []couleeTrigger {
	t.Helper()
	var doc struct {
		Triggers []couleeTrigger `yaml:"triggers"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(readQuestFile(t, file)), &doc))
	return doc.Triggers
}

// couleeActorLine is the line the trigger granting token prints.
func couleeActorLine(t *testing.T, file, token string) string {
	t.Helper()
	for _, tr := range couleeTriggers(t, file) {
		granted, line := false, ""
		for _, a := range tr.Actions {
			granted = granted || a.Grant == token
			line += a.Actor
		}
		if granted {
			return squash(line)
		}
	}
	t.Fatalf("%s has no trigger granting %s", file, token)
	return ""
}

func couleeStepDescription(t *testing.T, q *QuestDef, step string) string {
	t.Helper()
	for _, s := range q.Steps {
		if s.Id == step {
			return squash(s.Description)
		}
	}
	t.Fatalf("quest %d has no step %q", q.QuestId, step)
	return ""
}

// couleeOpenerOn is what an NPC says to a player on the quest's step.
func couleeOpenerOn(t *testing.T, file string, q *QuestDef, step string, pre ...string) (string, string) {
	t.Helper()
	text, hints := newNpcTalker(loadCouleeTopics(t, file), playerOn(q, step, pre...)).opener()
	return squash(text), squash(hints)
}

// Grieve says "wait a little" for a new mote, and a second casting can scatter
// it, so the mote comes back at the pace the Straw butt and the sisters do.
func TestPracticeMoteReturnsAtTheRateGrievePromises(t *testing.T) {
	spawners := couleeRoomsSpawning(t, 9146)
	require.Len(t, spawners, 1)
	require.Equal(t, foldingStarChamber, spawners[0].RoomId)
	s := spawners[0].SpawnInfo[0]
	require.Equal(t, 9146, s.MobId)
	require.Equal(t, "5 rounds", s.RespawnRate)
	require.True(t, s.NarrateRespawn, "the student in the chamber sees it come back")
	require.NotEmpty(t, s.Message)
}

// Any wisp grants Tending the Grove's kill, and each returns on the default
// rate of a quarter hour. So each line sending the player out names the rooms
// that spawn one and says it may take a while.
func TestQuest57LinesNameEveryWispRoomAndTheWait(t *testing.T) {
	f := "57-tending_the_grove.yaml"
	q := loadShippedQuest(t, f, 57)
	pre := []string{"44-end", "45-end", "46-end"}

	var titles []string
	for _, r := range couleeRoomsSpawning(t, foldingDriftWisp) {
		titles = append(titles, strings.ToLower(r.Title))
	}
	require.Len(t, titles, 3, "the lines below name each room that spawns a wisp")

	killTriggers := 0
	for _, tr := range couleeTriggers(t, f) {
		if tr.Event == "mob_death" {
			killTriggers++
			require.Equal(t, foldingDriftWisp, tr.Mob)
			require.Zero(t, tr.Room, "any wisp counts, so the kill names no room")
			require.Empty(t, tr.Area)
			require.Empty(t, tr.Zone)
		}
	}
	require.Equal(t, 1, killTriggers)

	orrin := loadCouleeTopics(t, "9145.yaml")
	openerText, _ := couleeOpenerOn(t, "9145.yaml", q, "start", pre...)
	lines := map[string]string{
		"step description": couleeStepDescription(t, q, "start"),
		"Orrin's opener":   openerText,
		"Orrin's offer":    squash(orrin.FindTopic("grove_loop_offer").Text),
		"Orrin's answer":   squash(orrin.FindTopic("grove_loop_mid").Text),
	}
	for name, line := range lines {
		for _, title := range titles {
			require.Contains(t, line, title, name)
		}
		require.Contains(t, line, "wait a while", name)
	}

	// A wisp killed at the Sunken Path or in the Thinning Wood is not south of
	// Orrin, so no line after the kill says which way he lies.
	require.NotContains(t, questHintFor(t, q, "kill"), "go back north")
	require.NotContains(t, couleeActorLine(t, f, "57-kill"), "go back north")
	require.NotContains(t, squash(orrin.FindTopic("grove_loop_turnin").PlayerLine), "grove edge")
}

// The Unfolded has one spawn on the default rate. Quest 52's Skell is the
// example: name the place and say that waiting brings it back.
func TestQuest46LinesSayTheUnfoldedComesBack(t *testing.T) {
	f := "46-the_unfolded.yaml"
	q := loadShippedQuest(t, f, 46)
	spawners := couleeRoomsSpawning(t, 9152)
	require.Len(t, spawners, 1)
	require.Equal(t, foldingThinnestPlace, spawners[0].RoomId)
	require.Empty(t, spawners[0].SpawnInfo[0].RespawnRate, "the default rate is why they wait")

	orrin := loadCouleeTopics(t, "9145.yaml")
	openerText, _ := couleeOpenerOn(t, "9145.yaml", q, "start", "44-end", "45-end")
	for name, line := range map[string]string{
		"step description": couleeStepDescription(t, q, "start"),
		"Orrin's opener":   openerText,
		"Orrin's answer":   squash(orrin.FindTopic("scab_mid").Text),
	} {
		require.Contains(t, line, "wait a while", name)
	}
}

// Quest 45's kill is granted by a wisp or a revenant, so the report fits both.
func TestGroveHandInFitsEitherKill(t *testing.T) {
	mobsKilled := map[int]bool{}
	for _, tr := range couleeTriggers(t, "45-the_unquiet_grove.yaml") {
		if tr.Event == "mob_death" {
			mobsKilled[tr.Mob] = true
		}
	}
	require.Equal(t, map[int]bool{foldingDriftWisp: true, foldingGroveRevenant: true}, mobsKilled)

	turnin := loadCouleeTopics(t, "9145.yaml").FindTopic("grove_turnin")
	require.NotContains(t, squash(turnin.PlayerLine), "revenant")
	require.NotContains(t, squash(turnin.PlayerLine), "wisp")
}

// A shot starts a fight, and sneak is refused in a fight until the butt falls,
// so the lines that say "type sneak" say that first.
func TestQuest50SneakLinesSayTheButtMustFallFirst(t *testing.T) {
	f := "50-first_shot.yaml"
	q := loadShippedQuest(t, f, 50)
	iden := loadCouleeTopics(t, "9161.yaml")
	_, openerHint := couleeOpenerOn(t, "9161.yaml", q, "shoot", "30-end")
	for name, line := range map[string]string{
		"shot line":   couleeActorLine(t, f, "50-shoot"),
		"step hint":   questHintFor(t, q, "shoot"),
		"Iden opener": openerHint,
		"Iden answer": squash(iden.FindTopic("coach_shoot").Hints),
	} {
		require.Contains(t, line, "type sneak", name)
		require.Contains(t, line, "butt", name)
		require.True(t, strings.Contains(line, "fall") || strings.Contains(line, "down"),
			"%s must say the butt has to be down: %q", name, line)
	}
}

// The taunt event fires after a resisted taunt too, so its line claims nothing
// about how the words landed.
func TestQuest48TauntLineClaimsNoOutcome(t *testing.T) {
	line := couleeActorLine(t, "48-the_standing_stones.yaml", "48-taunt")
	for _, claim := range []string{"gap", "straight through", "fight goes out", "surrender", "quits"} {
		require.NotContains(t, line, claim)
	}
	require.Contains(t, line, "type talk hale")
}

// Wenna's route to the Stone Circle does not cross the green: it lies north of
// the circle (the room data) and the route ends at the circle.
func TestWennaDoesNotPutTheGreenOnTheWayToHale(t *testing.T) {
	q := loadShippedQuest(t, "48-the_standing_stones.yaml", 48)
	text, hint := couleeOpenerOn(t, "9153.yaml", q, "start", "30-end", "47-end")
	require.Contains(t, hint, "into the stone circle")
	require.NotContains(t, hint, "green")
	require.NotContains(t, text, "on your way")
	require.Contains(t, text, "north of")
}

// A teacher's first lesson heads the menu's information lines, and no marked
// help line comes before it, because only the lesson earns the step. Every
// Coulee lesson that a topic_heard trigger counts is held to it.
func TestLessonsComeBeforeMarkedHelpOnEachTeachersMenu(t *testing.T) {
	for _, tc := range []struct {
		file, lesson string
	}{
		{"9144.yaml", "cast_lesson"},
		{"9153.yaml", "words_lesson"},
		{"9161.yaml", "shoot_lesson"},
	} {
		df := loadCouleeTopics(t, tc.file)
		require.Equal(t, tc.lesson, df.Topics[0].Key, tc.file)
	}

	entries, err := os.ReadDir(filepath.Join(couleeWorld(t), "quests"))
	require.NoError(t, err)
	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		var doc struct {
			Triggers []couleeTrigger `yaml:"triggers"`
		}
		if yaml.Unmarshal([]byte(readQuestFile(t, e.Name())), &doc) != nil {
			continue
		}
		for _, tr := range doc.Triggers {
			if tr.Event != "topic_heard" {
				continue
			}
			path := filepath.Join(couleeWorld(t), "dialogue", "pothole_coulee", fmt.Sprintf("%d.yaml", tr.Mob))
			if _, err := os.Stat(path); err != nil {
				continue // another zone's teacher
			}
			checked++
			requireNoMarkedInfoBefore(t, loadCouleeTopics(t, fmt.Sprintf("%d.yaml", tr.Mob)), tr.Topic)
		}
	}
	require.GreaterOrEqual(t, checked, 3, "the walk must reach the three teachers above")
}

func requireNoMarkedInfoBefore(t *testing.T, df *dialogue.DialogueFile, key string) {
	t.Helper()
	for i := range df.Topics {
		tp := &df.Topics[i]
		if tp.Key == key {
			return
		}
		// A marked line behind a quest gate does not sit beside a first lesson.
		if tp.Kind == dialogue.KindInfo && tp.Marked && len(tp.QuestRequired) == 0 {
			t.Errorf("mob %d: marked help %q comes before the lesson %q", df.MobId, tp.Key, key)
		}
	}
	t.Errorf("mob %d: lesson %q is not in the file", df.MobId, key)
}
