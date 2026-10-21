package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// The topic-side replacement for the mid-quest giver checks in
// newbie_alchemy_steps_test.go (Birna, Falv) and newbie_ranged_steps_test.go
// (Iden), which drive the pre-#375 ask engine (TreeAdvance, Greet, the
// catch-all) and cannot survive the switch to talk. Under talk the same
// promise reads: at every step the NPC's opener hint names the step's one
// command, an eligible info topic names it too, and on the step before the
// end the quest hint teaches `talk <npc>`, the opener names the hand-in by its
// label, and choosing the hand-in completes the quest.
//
// The shipped dialogue stays in the old format until the converted files land
// (#375 Task 15 step 4), so these skip while a file has no topics. Point
// DOGMUD_CONVERTED_DIALOGUE at a converted dialogue root (the folder holding
// pothole_coulee/) to run them against the converter's output before then.

func loadCouleeTopics(t *testing.T, file string) *dialogue.DialogueFile {
	t.Helper()
	root := os.Getenv("DOGMUD_CONVERTED_DIALOGUE")
	if root == "" {
		_, here, _, ok := runtime.Caller(0)
		require.True(t, ok)
		root = filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "dialogue")
	}
	raw, err := os.ReadFile(filepath.Join(root, "pothole_coulee", file))
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml.Unmarshal(raw, &df), "dialogue %s must parse, or the NPC goes mute", file)
	if len(df.Topics) == 0 {
		require.NotNil(t, df.Tree, "dialogue %s has neither topics nor a tree", file)
		t.Skipf("dialogue %s is still the pre-#375 format; the converted file lands in Task 15 step 4", file)
	}
	return &df
}

type topicStep struct {
	file   string
	id     int
	step   string
	want   string   // the step's one command
	others []string // prerequisite tokens
	final  bool     // the step before end: the hand-in
}

func checkTopicSteps(t *testing.T, npc, dfile string, cases []topicStep) {
	t.Helper()
	df := loadCouleeTopics(t, dfile)
	for _, c := range cases {
		q := loadShippedQuest(t, c.file, c.id)
		p := playerOn(q, c.step, c.others...)
		ps := &dialogue.PlayerState{
			HasQuest:    p.HasQuest,
			GiveQuest:   p.grant,
			HasItem:     func(int) bool { return false },
			CarriesItem: func(int) bool { return false },
			GiveItem:    func(int) bool { return true },
			RemoveItem:  func(int) bool { return true },
			HasHeard:    func(string) bool { return false },
			MarkHeard:   func(string) {},
		}
		_, opener := df.OpenerFor(ps)

		if !c.final {
			require.Contains(t, squash(questHintFor(t, q, c.step)), c.want,
				"quest %d step %q hint changed; update this test and %s together", c.id, c.step, npc)
			require.Contains(t, squash(opener), c.want,
				"quest %d step %q: %s's opener hint must name the step's command", c.id, c.step, npc)
			found := false
			for i := range df.Topics {
				tp := &df.Topics[i]
				if tp.Kind == dialogue.KindInfo && df.Eligible(tp, ps) && strings.Contains(squash(tp.Hints), c.want) {
					found = true
					break
				}
			}
			require.True(t, found, "quest %d step %q: no eligible %s topic names %q", c.id, c.step, npc, c.want)
			continue
		}

		require.Contains(t, squash(questHintFor(t, q, c.step)), "type talk "+npc,
			"quest %d step %q: the hand-in step's quest hint must teach talk", c.id, c.step)
		var handin *dialogue.Topic
		for i := range df.Topics {
			tp := &df.Topics[i]
			if tp.Kind == dialogue.KindHandIn && tp.Quest == c.id && df.Eligible(tp, ps) {
				handin = tp
				break
			}
		}
		require.NotNil(t, handin, "quest %d step %q: %s offers no eligible hand-in", c.id, c.step, npc)
		require.Contains(t, squash(opener), "choose "+strings.ToLower(handin.Label),
			"quest %d step %q: %s's opener hint must name the hand-in", c.id, c.step, npc)
		require.True(t, df.Apply(handin, ps), "quest %d: the hand-in did not apply", c.id)
		end := handin.GrantsQuest
		require.True(t, strings.HasSuffix(end, "-end"), "quest %d: the hand-in grants %q, not the end token", c.id, end)
		require.True(t, p.HasQuest(end), "quest %d: choosing the hand-in must complete it", c.id)
	}
}

// Rusk's First Heat hand-in (batch 1 review M7): a player who still carries
// the dagger shows it (craft_turnin, what `give` resolves to), and a player who
// sold or lost it reports on their word (craft_turnin_told). Exactly one is
// eligible either way, and both complete quest 35 without taking anything.
func TestRusk9116_HandInWithOrWithoutTheDagger(t *testing.T) {
	const ironDagger = 10009
	df := loadCouleeTopics(t, "9116.yaml")
	q := loadShippedQuest(t, "35-first_heat.yaml", 35)
	for _, carries := range []bool{true, false} {
		p := playerOn(q, "craft", "30-end")
		removed := false
		ps := &dialogue.PlayerState{
			HasQuest:    p.HasQuest,
			GiveQuest:   p.grant,
			HasItem:     func(id int) bool { return carries && id == ironDagger },
			CarriesItem: func(id int) bool { return carries && id == ironDagger },
			RemoveItem:  func(int) bool { removed = true; return true },
			GiveItem:    func(int) bool { return true },
			HasHeard:    func(string) bool { return false },
			MarkHeard:   func(string) {},
		}
		var eligible []*dialogue.Topic
		for i := range df.Topics {
			tp := &df.Topics[i]
			if tp.Kind == dialogue.KindHandIn && tp.Quest == 35 && df.Eligible(tp, ps) {
				eligible = append(eligible, tp)
			}
		}
		require.Len(t, eligible, 1, "carries dagger=%v: exactly one First Heat hand-in must be eligible", carries)
		want := "craft_turnin_told"
		if carries {
			want = "craft_turnin"
			byGive, _ := df.ItemTopic(ironDagger, ps)
			require.NotNil(t, byGive, "giving the dagger must resolve to the shown hand-in")
			require.Equal(t, "craft_turnin", byGive.Key)
		}
		require.Equal(t, want, eligible[0].Key, "carries dagger=%v", carries)
		require.True(t, df.Apply(eligible[0], ps))
		require.True(t, p.HasQuest("35-end"), "carries dagger=%v: the hand-in must complete First Heat", carries)
		require.False(t, removed, "carries dagger=%v: Rusk never keeps the dagger", carries)
	}
}

func TestBirna9128_TopicsNameEachStep(t *testing.T) {
	f := "38-first_brew.yaml"
	checkTopicSteps(t, "birna", "9128.yaml", []topicStep{
		{f, 38, "start", "type list", []string{"30-end"}, false},
		{f, 38, "list", "type buy glass vial", []string{"30-end"}, false},
		{f, 38, "vial", "type buy 2 healers root", []string{"30-end"}, false},
		{f, 38, "roots", "type craft healing salve", []string{"30-end"}, false},
		{f, 38, "brew", "type drink healing salve", []string{"30-end"}, false},
		{f, 38, "drink", "", []string{"30-end"}, true},
	})
}

func TestFalv9129_TopicsNameEachStep(t *testing.T) {
	f39 := "39-wade_the_marsh.yaml"
	checkTopicSteps(t, "falv", "9129.yaml", []topicStep{
		{f39, 39, "start", "type attack leech", []string{"38-end"}, false},
		{f39, 39, "leech", "type throw firebomb", []string{"38-end"}, false},
		{f39, 39, "throw", "type forage", []string{"38-end"}, false},
		{f39, 39, "forage", "", []string{"38-end"}, true},
		{"40-heart_of_the_mire.yaml", 40, "start", "type attack spirit", []string{"38-end", "39-end"}, false},
		{"55-another_brew.yaml", 55, "start", "type craft healing salve", []string{"38-end", "39-end", "40-end"}, false},
		{"55-another_brew.yaml", 55, "brew", "type drink healing salve", []string{"38-end", "39-end", "40-end"}, false},
	})
}

func TestIden9161_TopicsNameEachStep(t *testing.T) {
	f := "50-first_shot.yaml"
	checkTopicSteps(t, "iden", "9161.yaml", []topicStep{
		{f, 50, "start", "type equip sling", []string{"30-end"}, false},
		{f, 50, "equip", "type fire butt", []string{"30-end"}, false},
		{f, 50, "shoot", "type sneak", []string{"30-end"}, false},
		{f, 50, "ambush", "type fire butt", []string{"30-end"}, false},
		{f, 50, "cover_shot", "", []string{"30-end"}, true},
	})
}
