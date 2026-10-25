package questengine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yamlv2 "gopkg.in/yaml.v2"
)

// The talk-side checks for every Pothole Coulee giver (#375 phase 1). Before
// the switch to talk, the per-quest tests drove the ask engine (TreeAdvance,
// Greet, the catch-all) with the words a mid-quest player types. Under talk
// the same promise reads: at every step the NPC's opener hint names the step's
// one command, an eligible quest-aware info topic names it too (choosing it
// does not move the quest), and on the step before the end the quest hint
// teaches `talk <npc>`, the opener names the hand-in by its label, and
// choosing the hand-in completes the quest.

// loadShippedDialogue parses a shipped dialogue file with yaml.v2, the
// library the game's loader uses, so a file that would mute its NPC in game
// fails here too. Every shipped file is in the topic format since #375 Task
// 15, so a file without topics is a failure.
func loadShippedDialogue(t *testing.T, zoneDir, file string) *dialogue.DialogueFile {
	t.Helper()
	raw := repoFile(t, "_datafiles", "world", "dogmud", "dialogue", zoneDir, file)
	var df dialogue.DialogueFile
	require.NoError(t, yamlv2.Unmarshal(raw, &df), "dialogue %s/%s must parse, or the NPC goes mute", zoneDir, file)
	require.NotEmpty(t, df.Topics, "dialogue %s/%s has no topics", zoneDir, file)
	return &df
}

func loadCouleeTopics(t *testing.T, file string) *dialogue.DialogueFile {
	t.Helper()
	return loadShippedDialogue(t, "pothole_coulee", file)
}

// npcTalker is one player's view of an NPC's topics: quest progress from a
// linearPlayer, carried items in its item set, and every item handed out,
// taken and every topic heard recorded.
type npcTalker struct {
	df      *dialogue.DialogueFile
	p       *linearPlayer
	given   []int
	removed []int
	heard   map[string]bool
}

func newNpcTalker(df *dialogue.DialogueFile, p *linearPlayer) *npcTalker {
	return &npcTalker{df: df, p: p, heard: map[string]bool{}}
}

func (n *npcTalker) state() *dialogue.PlayerState {
	carries := func(id int) bool { return n.p.items[id] }
	return &dialogue.PlayerState{
		QuestOpen:   func(int) bool { return true },
		HasQuest:    n.p.HasQuest,
		GiveQuest:   n.p.grant,
		HasItem:     carries,
		CarriesItem: carries,
		GiveItem: func(id int) bool {
			n.given = append(n.given, id)
			n.p.items[id] = true
			return true
		},
		RemoveItem: func(id int) bool {
			if !n.p.items[id] {
				return false
			}
			delete(n.p.items, id)
			n.removed = append(n.removed, id)
			return true
		},
		GetQuestFlag: func(k string) string { return n.p.flags[k] },
		SetQuestFlag: func(k, v string) { n.p.flags[k] = v },
		HasHeard:     func(k string) bool { return n.heard[k] },
		MarkHeard:    func(k string) { n.heard[k] = true },
	}
}

// opener is what `talk <npc>` opens with for this player.
func (n *npcTalker) opener() (text, hints string) {
	return n.df.OpenerFor(n.state())
}

// eligible returns the topic with key when it is on the player's menu now.
func (n *npcTalker) eligible(key string) *dialogue.Topic {
	tp := n.df.FindTopic(key)
	if tp == nil || !n.df.Eligible(tp, n.state()) {
		return nil
	}
	return tp
}

// choose plays key the way the menu does: it must be eligible, then Apply
// runs its effects.
func (n *npcTalker) choose(t *testing.T, key string) *dialogue.Topic {
	t.Helper()
	require.NotNil(t, n.df.FindTopic(key), "dialogue %d has no topic %q", n.df.MobId, key)
	tp := n.eligible(key)
	require.NotNil(t, tp, "dialogue %d topic %q is not on the menu", n.df.MobId, key)
	require.True(t, n.df.Apply(tp, n.state()), "dialogue %d topic %q did not apply", n.df.MobId, key)
	return tp
}

// typedCount counts the commands a hint names, leaving out the dark aside and
// the help pointer (neither is the step).
func typedCount(hints string) int {
	h := strings.ReplaceAll(squash(hints), squash(darkAside), "")
	h = helpPointer.ReplaceAllString(h, "")
	return len(typedCommand.FindAllString(h, -1))
}

// gatesOnQuest reports whether a topic's own gates name a token of quest id:
// a quest-aware answer, not a general one.
func gatesOnQuest(tp *dialogue.Topic, id int) bool {
	prefix := fmt.Sprintf("%d-", id)
	for _, tok := range tp.QuestRequired {
		if strings.HasPrefix(tok, prefix) {
			return true
		}
	}
	return false
}

type topicStep struct {
	file   string
	id     int
	step   string
	want   string   // the step's one command (unused on the hand-in step)
	others []string // prerequisite tokens
	key    string   // the topic that answers this step: its progress topic, or the hand-in
	final  bool     // the step before end: the hand-in
}

func checkTopicSteps(t *testing.T, npc, dfile string, cases []topicStep) {
	t.Helper()
	checkTopicStepsWith(t, npc, dfile, nil, cases)
}

// checkTopicStepsWith runs checkTopicSteps with setup applied to each case's
// player first (quest flags, carried items).
func checkTopicStepsWith(t *testing.T, npc, dfile string, setup func(*linearPlayer), cases []topicStep) {
	t.Helper()
	df := loadCouleeTopics(t, dfile)
	for _, c := range cases {
		q := loadShippedQuest(t, c.file, c.id)
		p := playerOn(q, c.step, c.others...)
		if setup != nil {
			setup(p)
		}
		n := newNpcTalker(df, p)
		_, opener := n.opener()

		if !c.final {
			require.Contains(t, squash(questHintFor(t, q, c.step)), c.want,
				"quest %d step %q hint changed; update this test and %s together", c.id, c.step, npc)
			require.Contains(t, squash(opener), c.want,
				"quest %d step %q: %s's opener hint must name the step's command", c.id, c.step, npc)
			require.Equal(t, 1, typedCount(opener),
				"quest %d step %q: %s's opener hint names one command: %q", c.id, c.step, npc, opener)
			for i := range df.Topics {
				if h := &df.Topics[i]; h.Kind == dialogue.KindHandIn && h.Quest == c.id {
					require.NotContains(t, squash(opener), "choose "+strings.ToLower(h.Label),
						"quest %d step %q: %s's opener previews the hand-in before the last step", c.id, c.step, npc)
					require.Nil(t, n.eligible(h.Key), "quest %d step %q: hand-in %s is open early", c.id, c.step, h.Key)
				}
			}
			tp := n.eligible(c.key)
			require.NotNil(t, tp, "quest %d step %q: %s topic %q is not on the menu", c.id, c.step, npc, c.key)
			require.Equal(t, dialogue.KindInfo, tp.Kind, "quest %d step %q: %s", c.id, c.step, c.key)
			require.True(t, gatesOnQuest(tp, c.id), "quest %d step %q: %s topic %q must be quest-aware", c.id, c.step, npc, c.key)
			require.Contains(t, squash(tp.Hints), c.want,
				"quest %d step %q: %s topic %q must name the step's command", c.id, c.step, npc, c.key)
			n.choose(t, c.key)
			require.Equal(t, c.step, n.p.current, "quest %d step %q: asking %s about it must not move the quest", c.id, c.step, npc)
			continue
		}

		require.Contains(t, squash(questHintFor(t, q, c.step)), "type talk "+npc,
			"quest %d step %q: the hand-in step's quest hint must teach talk", c.id, c.step)
		handin := n.eligible(c.key)
		require.NotNil(t, handin, "quest %d step %q: %s hand-in %q is not on the menu", c.id, c.step, npc, c.key)
		require.Equal(t, dialogue.KindHandIn, handin.Kind)
		require.Equal(t, c.id, handin.Quest)
		require.Contains(t, squash(opener), "choose "+strings.ToLower(handin.Label),
			"quest %d step %q: %s's opener hint must name the hand-in", c.id, c.step, npc)
		n.choose(t, c.key)
		end := handin.GrantsQuest
		require.True(t, strings.HasSuffix(end, "-end"), "quest %d: the hand-in grants %q, not the end token", c.id, end)
		require.True(t, n.p.HasQuest(end), "quest %d: choosing the hand-in must complete it", c.id)
	}
}

// checkHandout: a replacement topic gives its item once. A player without one
// sees the replacement, takes it, and from then on sees only the reminder,
// which hands nothing out (S8).
func checkHandout(t *testing.T, n *npcTalker, supply, have string, item int) {
	t.Helper()
	sup := n.df.FindTopic(supply)
	require.NotNil(t, sup, "no topic %q", supply)
	require.Equal(t, item, sup.GivesItem, "%s hands out the item", supply)
	require.Equal(t, item, sup.ItemExcluded, "%s must hide from a player who carries one", supply)
	hv := n.df.FindTopic(have)
	require.NotNil(t, hv, "no topic %q", have)
	require.Equal(t, item, hv.CarriesItem, "%s shows only to a player who carries one", have)
	require.Zero(t, hv.GivesItem, "%s must not hand anything out", have)
	require.Equal(t, sup.QuestRequired, hv.QuestRequired, "%s and %s answer the same steps", supply, have)
	require.Equal(t, sup.QuestExcluded, hv.QuestExcluded, "%s and %s answer the same steps", supply, have)

	require.Nil(t, n.eligible(have), "a player without one sees no reminder")
	n.choose(t, supply)
	require.Equal(t, []int{item}, n.given, "a player without one gets one")
	require.Nil(t, n.eligible(supply), "a player who carries one must not be offered a second")
	n.choose(t, have)
	require.Equal(t, []int{item}, n.given, "a player who carries one must not get a second")
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
			QuestOpen:   func(int) bool { return true },
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
	pre := []string{"30-end"}
	checkTopicSteps(t, "birna", "9128.yaml", []topicStep{
		{f, 38, "start", "type list", pre, "brew_step_list", false},
		{f, 38, "list", "type buy glass vial", pre, "brew_step_vial", false},
		{f, 38, "vial", "type buy 2 healers root", pre, "brew_step_roots", false},
		{f, 38, "roots", "type craft healing salve", pre, "brew_step_craft", false},
		{f, 38, "brew", "type drink healing salve", pre, "brew_step_drink", false},
		{f, 38, "drink", "", pre, "brew_turnin", true},
	})
}

func TestFalv9129_TopicsNameEachStep(t *testing.T) {
	f39 := "39-wade_the_marsh.yaml"
	checkTopicSteps(t, "falv", "9129.yaml", []topicStep{
		{f39, 39, "start", "type attack leech", []string{"38-end"}, "marsh_step_leech", false},
		{f39, 39, "leech", "type throw firebomb", []string{"38-end"}, "marsh_step_throw", false},
		{f39, 39, "throw", "type forage", []string{"38-end"}, "marsh_step_forage", false},
		{f39, 39, "forage", "", []string{"38-end"}, "marsh_turnin", true},
		{"40-heart_of_the_mire.yaml", 40, "start", "type attack spirit", []string{"38-end", "39-end"}, "mire_step_help", false},
		{"55-another_brew.yaml", 55, "start", "type craft healing salve", []string{"38-end", "39-end", "40-end"}, "brew_loop_step_craft", false},
		{"55-another_brew.yaml", 55, "brew", "type drink healing salve", []string{"38-end", "39-end", "40-end"}, "brew_loop_step_drink", false},
	})
}

func TestIden9161_TopicsNameEachStep(t *testing.T) {
	f := "50-first_shot.yaml"
	pre := []string{"30-end"}
	checkTopicSteps(t, "iden", "9161.yaml", []topicStep{
		{f, 50, "start", "type equip sling", pre, "coach_start", false},
		{f, 50, "equip", "type fire butt", pre, "coach_equip", false},
		{f, 50, "shoot", "type sneak", pre, "coach_shoot", false},
		{f, 50, "ambush", "type fire butt", pre, "coach_ambush", false},
		{f, 50, "cover_shot", "", pre, "shoot_turnin", true},
	})
}
