package quests

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// branchedQuest is quest 11's shape (#958): one fork on the branch flag, two
// steps a side, shared start and end.
func branchedQuest() *Quest {
	return &Quest{
		QuestId: 97311,
		Name:    "Forked",
		Flags:   []QuestFlagDef{{Key: "branch", Values: []string{"sylara", "rhett"}}},
		Steps: []QuestStep{
			{Id: "start"},
			{Id: "sylara", Branch: "sylara"},
			{Id: "totem", Branch: "sylara"},
			{Id: "rhett", Branch: "rhett"},
			{Id: "samples", Branch: "rhett"},
			{Id: "end"},
		},
	}
}

func plainQuest() *Quest {
	return &Quest{
		QuestId: 97312,
		Name:    "Straight",
		Flags:   []QuestFlagDef{{Key: "mood", Values: []string{"calm"}}},
		Steps:   []QuestStep{{Id: "start"}, {Id: "middle"}, {Id: "later"}, {Id: "end"}},
	}
}

// flags makes a flag reader over registered keys ("<questId>-<key>").
func flags(kv ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return func(k string) string { return m[k] }
}

func stepIds(steps []QuestStep) string {
	ids := make([]string, len(steps))
	for i, s := range steps {
		ids[i] = s.Id
	}
	return strings.Join(ids, ",")
}

func TestQuestStep_BranchLoads(t *testing.T) {
	var q Quest
	require.NoError(t, yaml.Unmarshal([]byte("questid: 1\nname: x\nsteps:\n  - id: start\n  - id: a\n    branch: one\n"), &q))
	require.Equal(t, "one", q.Steps[1].Branch)

	out, err := yaml.Marshal(QuestStep{Id: "start"})
	require.NoError(t, err)
	require.NotContains(t, string(out), "branch", "an unbranched step writes no branch key")
}

func TestQuest_PathAndProgress(t *testing.T) {
	b, p := branchedQuest(), plainQuest()
	rhettFlag := flags("97311-branch", "rhett")
	sylaraFlag := flags("97311-branch", "sylara")

	cases := []struct {
		name        string
		q           *Quest
		step        string
		flag        func(string) string
		path        string
		done, total int
	}{
		{"plain: every step, flag ignored", p, "middle", flags("97312-mood", "calm"), "start,middle,later,end", 2, 4},
		{"plain: nil flag reader", p, "start", nil, "start,middle,later,end", 1, 4},
		{"plain: unknown step reads as done", p, "gone", nil, "start,middle,later,end", 4, 4},
		{"flag unset at start: unbranched only", b, "start", nil, "start,end", 1, 2},
		{"flag unset reader at start", b, "start", flags(), "start,end", 1, 2},
		{"rhett flag at start", b, "start", rhettFlag, "start,rhett,samples,end", 1, 4},
		{"rhett at rhett", b, "rhett", rhettFlag, "start,rhett,samples,end", 2, 4},
		{"sylara at totem", b, "totem", sylaraFlag, "start,sylara,totem,end", 3, 4},
		{"rhett flag at end", b, "end", rhettFlag, "start,rhett,samples,end", 4, 4},
		{"branched step wins over a disagreeing flag", b, "rhett", sylaraFlag, "start,rhett,samples,end", 2, 4},
		{"branched step wins over an unset flag", b, "samples", nil, "start,rhett,samples,end", 3, 4},
		{"undeclared flag value: unbranched only", b, "start", flags("97311-branch", "nobody"), "start,end", 1, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.path, stepIds(c.q.Path(c.step, c.flag)))
			done, total := c.q.StepProgress(c.step, c.flag)
			require.Equal(t, [2]int{c.done, c.total}, [2]int{done, total})
		})
	}
}

func TestQuest_Paths(t *testing.T) {
	var got []string
	for _, path := range branchedQuest().Paths() {
		got = append(got, stepIds(path))
	}
	require.Equal(t, []string{"start,sylara,totem,end", "start,rhett,samples,end"}, got, "one path per value, in declared order")

	paths := plainQuest().Paths()
	require.Len(t, paths, 1)
	require.Equal(t, "start,middle,later,end", stepIds(paths[0]))
}

func TestIsTokenAfterOnPath(t *testing.T) {
	t.Cleanup(SeedQuestsForTest(branchedQuest(), plainQuest(), &Quest{QuestId: 97313, Name: "One", Steps: []QuestStep{{Id: "end"}}}))
	rhett := flags("97311-branch", "rhett")
	sylara := flags("97311-branch", "sylara")

	cases := []struct {
		name          string
		current, next string
		flag          func(string) string
		want          bool
	}{
		// Plain quests behave as IsTokenAfter always did.
		{"plain forward", "97312-start", "97312-later", nil, true},
		{"plain backward", "97312-later", "97312-middle", nil, false},
		{"plain same", "97312-middle", "97312-middle", nil, false},
		{"plain unknown current", "97312-gone", "97312-end", nil, false},
		{"plain unknown next", "97312-start", "97312-gone", nil, false},
		{"other quest", "97312-start", "97311-end", nil, false},
		{"no progress takes start", "97312-", "97312-start", nil, true},
		{"no progress refuses later", "97312-", "97312-middle", nil, false},
		{"no progress ends a one-step quest", "97313-", "97313-end", nil, true},
		{"no progress cannot end a long quest", "97312-", "97312-end", nil, false},
		{"unknown quest", "1-start", "1-end", nil, false},

		// Branched quest.
		{"start with flag unset refuses a side", "97311-start", "97311-rhett", nil, false},
		{"start with flag unset may end", "97311-start", "97311-end", nil, true},
		{"start on rhett takes rhett", "97311-start", "97311-rhett", rhett, true},
		{"start on rhett refuses sylara", "97311-start", "97311-sylara", rhett, false},
		{"start on rhett refuses totem", "97311-start", "97311-totem", rhett, false},
		{"rhett to samples", "97311-rhett", "97311-samples", rhett, true},
		{"rhett refuses totem", "97311-rhett", "97311-totem", rhett, false},
		{"rhett to end", "97311-rhett", "97311-end", rhett, true},
		{"rhett step with sylara flag keeps its side", "97311-rhett", "97311-samples", sylara, true},
		{"rhett step with sylara flag still refuses totem", "97311-rhett", "97311-totem", sylara, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, IsTokenAfterOnPath(c.current, c.next, c.flag))
		})
	}
}

func TestIsTokenPassedOnPath(t *testing.T) {
	t.Cleanup(SeedQuestsForTest(branchedQuest(), plainQuest()))
	rhett := flags("97311-branch", "rhett")
	sylara := flags("97311-branch", "sylara")

	cases := []struct {
		name           string
		token, current string
		flag           func(string) string
		want           bool
	}{
		{"plain earlier step passed", "97312-middle", "97312-later", nil, true},
		{"plain later step not passed", "97312-end", "97312-later", nil, false},
		{"plain empty token at start", "97312-", "97312-start", nil, true},
		{"rhett at samples passed rhett", "97311-rhett", "97311-samples", rhett, true},
		{"rhett at samples passed start", "97311-start", "97311-samples", rhett, true},
		{"rhett at samples never held sylara", "97311-sylara", "97311-samples", rhett, false},
		{"rhett at end never held totem", "97311-totem", "97311-end", rhett, false},
		{"rhett at end passed samples", "97311-samples", "97311-end", rhett, true},
		{"sylara at end passed totem", "97311-totem", "97311-end", sylara, true},
		{"unsided at end held no side", "97311-sylara", "97311-end", nil, false},
		{"rhett step with sylara flag: start passed", "97311-start", "97311-samples", sylara, true},
		{"rhett step with sylara flag: rhett passed", "97311-rhett", "97311-samples", sylara, true},
		{"rhett step with sylara flag: sylara not held", "97311-sylara", "97311-samples", sylara, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, IsTokenPassedOnPath(c.token, c.current, c.flag))
		})
	}
}

// HasQuest runs on every trigger check: the on-path order must not allocate.
func TestOnPath_NoAllocation(t *testing.T) {
	b, p := branchedQuest(), plainQuest()
	rhett := func(string) string { return "rhett" }
	allocs := testing.AllocsPerRun(100, func() {
		_ = p.stepBefore("middle", "start", "middle", nil)
		_ = b.stepBefore("samples", "rhett", "samples", rhett)
		_, _ = b.StepProgress("totem", rhett)
	})
	require.Zero(t, allocs)
}

func TestQuest_ValidateBranches(t *testing.T) {
	require.NoError(t, branchedQuest().Validate())
	require.NoError(t, plainQuest().Validate())

	cases := []struct {
		name   string
		edit   func(q *Quest)
		reason string
	}{
		{"value of no flag", func(q *Quest) { q.Steps[1].Branch = "kael" }, `branch "kael" is not a value of any of the quest's flags`},
		{"value of two flags", func(q *Quest) {
			q.Flags = append(q.Flags, QuestFlagDef{Key: "ally", Values: []string{"rhett"}})
		}, `branch "rhett" is a value of more than one flag`},
		{"two flags across steps", func(q *Quest) {
			q.Flags = append(q.Flags, QuestFlagDef{Key: "ally", Values: []string{"kael"}})
			q.Steps[4].Branch = "kael"
		}, `branched steps use flags "branch" and "ally"`},
		{"branched first step", func(q *Quest) { q.Steps[0].Branch = "sylara" }, `first step "start" cannot be branched`},
		{"branched last step", func(q *Quest) { q.Steps[5].Branch = "rhett" }, `last step "end" cannot be branched`},
		{"repeatable", func(q *Quest) { q.Repeatable = true }, `a repeatable quest cannot branch`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := branchedQuest()
			c.edit(q)
			err := q.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), c.reason)
		})
	}
}
