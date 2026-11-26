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

// HasQuest runs on every trigger check: the on-path order must not allocate,
// on a branched step or an unbranched one (where the flag is read), for a
// quest that went through Validate as every loaded quest does.
func TestOnPath_NoAllocation(t *testing.T) {
	b, p := branchedQuest(), plainQuest()
	require.NoError(t, b.Validate())
	require.NoError(t, p.Validate())
	rhett := func(string) string { return "rhett" }
	none := func(string) bool { return false }
	allocs := testing.AllocsPerRun(100, func() {
		_ = p.stepBefore("middle", "start", "middle", nil)
		_ = b.stepBefore("samples", "rhett", "samples", rhett)
		_ = b.stepBefore("start", "start", "rhett", rhett)
		_ = b.stepBefore("end", "samples", "end", rhett)
		_, _ = b.StepProgress("totem", rhett)
		_, _ = b.StepProgress("start", rhett)
		_ = b.Side("end", rhett)
		_ = b.GrantOnPath("rhett", none, rhett)
	})
	require.Zero(t, allocs)
}

// GrantOnPath reads the side from what the player holds (the trigger engine
// sees a player only through HasQuest and its flags). It must agree with Side
// for every step and flag a player can stand on.
func TestQuest_GrantOnPathAgreesWithSide(t *testing.T) {
	b, p := branchedQuest(), plainQuest()
	t.Cleanup(SeedQuestsForTest(b, p))
	for _, q := range []*Quest{b, p} {
		for _, at := range q.Steps {
			for _, flagVal := range []string{"", "sylara", "rhett", "nobody"} {
				flag := flags(flagKey(q.QuestId, "branch"), flagVal)
				cur := PartsToToken(q.QuestId, at.Id)
				has := func(tok string) bool { return tok == cur || IsTokenPassedOnPath(tok, cur, flag) }
				side := q.Side(at.Id, flag)
				for _, next := range q.Steps {
					want := next.Branch == "" || next.Branch == side
					require.Equal(t, want, q.GrantOnPath(next.Id, has, flag),
						"quest %d at %s, flag %q, grant %s", q.QuestId, at.Id, flagVal, next.Id)
				}
			}
		}
	}
	require.True(t, b.GrantOnPath("gone", func(string) bool { return false }, nil), "an unknown step is not this gate's to refuse")
}

// A holder that reads steps as one straight line (the walk player, a test
// mock) "holds" every earlier step, Sylara's included, when it stands at
// rhett. The gate must still open Rhett's next step to it: holding a step of
// the grant's own side is enough.
func TestQuest_GrantOnPathNaiveHolder(t *testing.T) {
	b := branchedQuest()
	naive := func(held ...string) func(string) bool {
		return func(tok string) bool {
			for _, h := range held {
				if PartsToToken(b.QuestId, h) == tok {
					return true
				}
			}
			return false
		}
	}
	atRhett := naive("start", "sylara", "totem", "rhett")
	require.True(t, b.GrantOnPath("samples", atRhett, nil), "Rhett's return step stays open")

	atSylara := naive("start", "sylara")
	require.False(t, b.GrantOnPath("rhett", atSylara, flags("97311-branch", "rhett")),
		"holding only the other side's steps shuts this side, whatever the flag says")
	require.True(t, b.GrantOnPath("rhett", naive("start"), flags("97311-branch", "rhett")),
		"holding no branched step, the flag decides")
}

// A quest edited after Validate (a builder save, a test that renumbers it)
// must not read the fork Validate worked out for its old shape.
func TestQuest_BranchCacheFollowsEdits(t *testing.T) {
	b := branchedQuest()
	require.NoError(t, b.Validate())
	b.QuestId = 97399
	require.Equal(t, "97399-branch", b.BranchFlagKey(), "a renumbered quest reads its new key")

	p := plainQuest()
	require.NoError(t, p.Validate())
	p.Steps = append(p.Steps[:3], QuestStep{Id: "calm", Branch: "calm"}, p.Steps[3])
	require.Equal(t, "97312-mood", p.BranchFlagKey(), "a step added after Validate is read")
	require.Equal(t, "97312-calm", p.stepToken(3), "and so is its token")
}

// The step-id forms of the token helpers: a reader holding the quest (the
// walkthrough's player) asks the same order as Character does.
func TestQuest_StepOrderOnPath(t *testing.T) {
	b := branchedQuest()
	rhett := flags("97311-branch", "rhett")
	require.True(t, b.IsStepAfterOnPath("", "start", nil), "a quest is entered at start")
	require.False(t, b.IsStepAfterOnPath("", "sylara", nil), "and only at start")
	require.True(t, b.IsStepAfterOnPath("start", "rhett", rhett))
	require.False(t, b.IsStepAfterOnPath("start", "sylara", rhett), "never onto the other side")
	require.False(t, b.IsStepAfterOnPath("start", "rhett", nil), "never onto a side before the flag names it")
	require.False(t, b.IsStepAfterOnPath("rhett", "rhett", rhett))
	require.True(t, b.IsStepPassedOnPath("start", "samples", nil))
	require.False(t, b.IsStepPassedOnPath("totem", "samples", nil), "a Rhett player never holds Sylara's steps")
	require.False(t, b.IsStepPassedOnPath("samples", "samples", nil), "the current step is not passed")

	one := &Quest{QuestId: 97313, Steps: []QuestStep{{Id: "end"}}}
	require.True(t, one.IsStepAfterOnPath("", "end", nil), "a one-step quest is entered at its only step")
	require.Equal(t, []string{"sylara", "rhett"}, b.Sides())
	require.Equal(t, []string{""}, plainQuest().Sides())
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
		{"flag value no step uses", func(q *Quest) {
			q.Flags[0].Values = append(q.Flags[0].Values, "kael")
		}, `value "kael" of branch flag "branch" is the branch of no step`},
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
