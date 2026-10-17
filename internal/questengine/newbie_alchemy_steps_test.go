package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// The Alchemy spoke of Pothole Coulee (quests 38, 39 and 55) under the owner
// ruling of 2026-10-17: one command per step, steps in order, each trigger
// gated on the step before it. These walk the shipped YAML with a player whose
// progress behaves like a real character's (see linearPlayer).

const (
	glassVial    = 40006
	healersRoot  = 40004
	firebombItem = 30058
	marshLeech   = 9130
	sheltered    = 5265 // Birna's pool, an alchemy bench
	dryingRacks  = 5267 // the second alchemy bench
	sunkenPath   = 5270
	blackPool    = 5273
)

func TestQuest38FirstBrew_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "38-first_brew.yaml", 38)
	steps := newLinearPlayer(q).steps
	for _, old := range []string{"start", "brew", "drink", "end"} {
		require.Contains(t, steps, old, "step %q must survive so a player mid-quest resumes", old)
	}
	turnIn := turnInRequires(t, "9128.yaml", "brew_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("item_gain", EventDetails{ItemId: glassVial, RoomId: sheltered})
		require.Equal(t, "start", w.p.current, "a vial bought before listing must not skip the list step")

		w.notify("command", EventDetails{Command: "list", RoomId: sheltered})
		require.Equal(t, "list", w.p.current)

		w.notify("item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "list", w.p.current, "a root before the vial must not skip the vial step")

		w.notify("item_gain", EventDetails{ItemId: glassVial, RoomId: sheltered})
		require.Equal(t, "vial", w.p.current)

		w.notify("command", EventDetails{Command: "drink", RoomId: sheltered})
		require.Equal(t, "vial", w.p.current, "a drink before the brew must not count")

		w.notify("item_gain", EventDetails{ItemId: healersRoot, RoomId: sheltered})
		require.Equal(t, "roots", w.p.current)
		require.False(t, holdsAll(w.p, turnIn))

		w.notify("command", EventDetails{Command: "craft", RoomId: sheltered})
		require.Equal(t, "brew", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the drink")

		w.notify("command", EventDetails{Command: "drink", RoomId: sheltered})
		require.Equal(t, "drink", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	// Old saves: a player on start may already carry the vial and roots (the
	// old start step asked for all three purchases at once). Crafting needs
	// them in hand, so the brew grant is lenient on 38-start.
	t.Run("player saved on start with materials can craft", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "craft", RoomId: dryingRacks})
		require.Equal(t, "brew", w.p.current)
	})

	t.Run("player saved on start without materials lists first", func(t *testing.T) {
		w := newQuestWalker(q).at("start")
		w.notify("command", EventDetails{Command: "list", RoomId: sheltered})
		require.Equal(t, "list", w.p.current)
	})

	t.Run("player saved on brew resumes with the drink", func(t *testing.T) {
		w := newQuestWalker(q).at("brew")
		w.notify("command", EventDetails{Command: "drink", RoomId: 5269})
		require.Equal(t, "drink", w.p.current)
		require.True(t, holdsAll(w.p, turnIn))
	})

	t.Run("player saved on drink can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("drink")
		require.True(t, holdsAll(w.p, turnIn))
	})
}

func TestQuest39WadeTheMarsh_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "39-wade_the_marsh.yaml", 39)
	require.Equal(t, []string{"start", "leech", "throw", "forage", "end"}, newLinearPlayer(q).steps,
		"the step ids are what a saved character stores; keep them so a player mid-quest resumes")
	turnIn := turnInRequires(t, "9129.yaml", "marsh_turnin")

	t.Run("walk in order", func(t *testing.T) {
		w := newQuestWalker(q).at("start")

		w.notify("command", EventDetails{Command: "forage", RoomId: sunkenPath})
		require.Equal(t, "start", w.p.current, "a forage before the leech must not skip ahead")
		w.notify("command", EventDetails{Command: "throw", RoomId: 5269})
		require.Equal(t, "start", w.p.current, "a throw before the leech is down must not skip the leech step")

		w.notify("mob_death", EventDetails{MobId: marshLeech, RoomId: 5269})
		require.Equal(t, "leech", w.p.current)

		w.notify("command", EventDetails{Command: "forage", RoomId: blackPool})
		require.Equal(t, "leech", w.p.current, "a forage before the throw must not skip the throw step")

		w.notify("command", EventDetails{Command: "throw", RoomId: 5272})
		require.Equal(t, "throw", w.p.current)
		require.False(t, holdsAll(w.p, turnIn), "turn-in must wait for the forage")

		w.notify("command", EventDetails{Command: "forage", RoomId: sunkenPath})
		require.Equal(t, "forage", w.p.current)
		require.True(t, holdsAll(w.p, turnIn), "turn-in opens after the last step")
	})

	t.Run("player saved on leech resumes with the throw", func(t *testing.T) {
		w := newQuestWalker(q).at("leech")
		w.notify("command", EventDetails{Command: "throw", RoomId: blackPool})
		require.Equal(t, "throw", w.p.current)
	})

	t.Run("player saved on throw resumes with the forage", func(t *testing.T) {
		w := newQuestWalker(q).at("throw")
		w.notify("command", EventDetails{Command: "forage", RoomId: blackPool})
		require.Equal(t, "forage", w.p.current)
	})

	t.Run("player saved on forage can turn in", func(t *testing.T) {
		w := newQuestWalker(q).at("forage")
		require.True(t, holdsAll(w.p, turnIn))
	})
}

func TestQuest55AnotherBrew_OneCommandPerStep(t *testing.T) {
	q := loadShippedQuest(t, "55-another_brew.yaml", 55)
	require.Equal(t, []string{"start", "brew", "end"}, newLinearPlayer(q).steps)

	w := newQuestWalker(q).at("start")
	w.notify("command", EventDetails{Command: "drink", RoomId: sheltered})
	require.Equal(t, "start", w.p.current, "a drink before the brew must not finish the quest")

	// Both benches in the coulee are alchemy benches.
	w.notify("command", EventDetails{Command: "craft", RoomId: dryingRacks})
	require.Equal(t, "brew", w.p.current, "a craft at the Drying Racks bench counts")

	w.notify("command", EventDetails{Command: "drink", RoomId: 5271})
	require.Equal(t, "end", w.p.current)
}

// Falv hands out a replacement firebomb while quest 39 is open. A player who
// still carries one must not get a second (S8): the handout node carries
// itemExcluded, and the node right after it answers the same words.
func TestFalvFirebombHandoutGivesOnlyOne(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", "9129.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var df struct {
		Tree struct {
			Nodes []struct {
				Id           string   `yaml:"id"`
				Triggers     []string `yaml:"triggers"`
				GivesItem    int      `yaml:"givesItem"`
				ItemExcluded int      `yaml:"itemExcluded"`
				GrantsQuest  string   `yaml:"grantsQuest"`
			} `yaml:"nodes"`
		} `yaml:"tree"`
	}
	require.NoError(t, yaml.Unmarshal(data, &df))

	found := false
	for i, n := range df.Tree.Nodes {
		if n.GivesItem != firebombItem || n.GrantsQuest != "" {
			continue
		}
		found = true
		require.Equal(t, firebombItem, n.ItemExcluded, "node %s hands a firebomb to a player who already carries one", n.Id)
		require.Less(t, i+1, len(df.Tree.Nodes), "node %s needs a refusal node after it", n.Id)
		next := df.Tree.Nodes[i+1]
		require.Equal(t, n.Triggers, next.Triggers, "the node after %s must answer the same words", n.Id)
		require.Zero(t, next.GivesItem, "the refusal after %s must not hand anything out", n.Id)
	}
	require.True(t, found, "no firebomb handout node found; the check would prove nothing")

	// Behaviour: a player without a firebomb gets one, a player with one
	// hears that they have one and gets nothing.
	q := loadShippedQuest(t, "39-wade_the_marsh.yaml", 39)
	falv := loadCouleeDialogue(t, "9129.yaml")
	n := newNpcTalker(falv, playerOn(q, "start", "38-end"))
	n.ask("firebomb")
	require.Equal(t, []int{firebombItem}, n.given, "a player without a firebomb gets one")
	txt, _, _, _ := n.ask("firebomb")
	require.Equal(t, []int{firebombItem}, n.given, "a player who carries one gets no second")
	require.Contains(t, squash(txt), "already have one")
}

type giverStep struct {
	file   string
	id     int
	step   string
	want   string   // the step's one command, as the greeting hint names it
	others []string // prerequisite tokens
	topics []string // words a mid-quest player asks
	final  bool     // the step before end, where "done" is the turn-in
}

func checkGiverSteps(t *testing.T, npc string, df string, cases []giverStep) {
	t.Helper()
	d := loadCouleeDialogue(t, df)
	for _, c := range cases {
		q := loadShippedQuest(t, c.file, c.id)
		require.Contains(t, questHintFor(t, q, c.step), c.want,
			"quest %d step %q hint changed; update this test and %s together", c.id, c.step, npc)
		n := newNpcTalker(d, playerOn(q, c.step, c.others...))
		_, hints := n.greet()
		require.Contains(t, squash(hints), c.want, "quest %d step %q: %s's greeting must name the step's command", c.id, c.step, npc)
		require.Equal(t, 1, len(typedCommand.FindAllString(strings.ReplaceAll(squash(hints), squash(darkAside), ""), -1)),
			"quest %d step %q greeting hint names one command: %q", c.id, c.step, hints)
		for _, topic := range c.topics {
			_, h, fromTree, catchAll := newNpcTalker(d, playerOn(q, c.step, c.others...)).ask(topic)
			require.False(t, catchAll, "quest %d step %q: ask %s %s falls to the catch-all", c.id, c.step, npc, topic)
			require.True(t, fromTree, "quest %d step %q: ask %s %s must reach a quest-aware node", c.id, c.step, npc, topic)
			if !c.final {
				require.Contains(t, squash(h), c.want, "quest %d step %q: ask %s %s must name the step's command", c.id, c.step, npc, topic)
			}
		}
	}
}

// Birna greets each quest 38 step with its command and answers the words the
// lesson invites (S10), including "bottle" for the glass vial.
func TestBirna9128_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	words := []string{"vial", "bottle", "root", "salve", "craft", "drink", "help", "quest", "done"}
	f := "38-first_brew.yaml"
	checkGiverSteps(t, "birna", "9128.yaml", []giverStep{
		{f, 38, "start", "type list", []string{"30-end"}, words, false},
		{f, 38, "list", "type buy glass vial", []string{"30-end"}, words, false},
		{f, 38, "vial", "type buy 2 healers root", []string{"30-end"}, words, false},
		{f, 38, "roots", "type craft healing salve", []string{"30-end"}, words, false},
		{f, 38, "brew", "type drink healing salve", []string{"30-end"}, words, false},
		{f, 38, "drink", "type ask birna done", []string{"30-end"}, []string{"vial", "salve", "drink", "done"}, true},
	})
}

// Falv greets each quest 39, 40 and 55 step with its command and answers the
// words the quests invite (S10).
func TestFalv9129_MidQuestGreetsAndAnswersEachStep(t *testing.T) {
	marsh := []string{"leech", "forage", "herbs", "throw", "help", "quest", "done"}
	f39 := "39-wade_the_marsh.yaml"
	checkGiverSteps(t, "falv", "9129.yaml", []giverStep{
		{f39, 39, "start", "type attack leech", []string{"38-end"}, marsh, false},
		{f39, 39, "leech", "type throw firebomb", []string{"38-end"}, append(marsh, "firebomb"), false},
		{f39, 39, "throw", "type forage", []string{"38-end"}, append(marsh, "firebomb"), false},
		{f39, 39, "forage", "type ask falv done", []string{"38-end"}, []string{"leech", "forage", "herbs", "done"}, true},
		{"40-heart_of_the_mire.yaml", 40, "start", "type attack spirit", []string{"38-end", "39-end"},
			[]string{"spirit", "mire", "where", "help", "quest"}, false},
		{"55-another_brew.yaml", 55, "start", "type craft healing salve", []string{"38-end", "39-end", "40-end"},
			[]string{"brew", "craft", "quest", "done"}, false},
		{"55-another_brew.yaml", 55, "brew", "type drink healing salve", []string{"38-end", "39-end", "40-end"},
			[]string{"brew", "drink", "quest", "done"}, false},
	})
}

// S5: Another Brew finishes on the drink, so nothing may send the player
// back to Falv to report it.
func TestQuest55AnotherBrew_NoReportBack(t *testing.T) {
	q := loadShippedQuest(t, "55-another_brew.yaml", 55)
	lines := []string{q.Description, q.Rewards.PlayerMessage}
	for _, s := range q.Steps {
		lines = append(lines, s.Hint, s.Description)
	}
	for _, tr := range q.Triggers {
		for _, a := range tr.Actions {
			lines = append(lines, a.SendText)
		}
	}
	for _, l := range lines {
		l = squash(l)
		require.NotContains(t, l, "come back", "quest 55 ends on the drink: %q", l)
		require.NotContains(t, l, "report back", "quest 55 ends on the drink: %q", l)
		require.NotContains(t, l, "tell him", "quest 55 ends on the drink: %q", l)
	}
}
