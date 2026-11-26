package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// stepQuests builds plain quests (no branched steps) from step ids, for the
// walk player.
func stepQuests(steps map[int][]string) map[int]*quests.Quest {
	out := map[int]*quests.Quest{}
	for id, ids := range steps {
		q := &quests.Quest{QuestId: id}
		for _, s := range ids {
			q.Steps = append(q.Steps, quests.QuestStep{Id: s})
		}
		out[id] = q
	}
	return out
}

func walkQuests() map[int]*quests.Quest {
	return stepQuests(map[int][]string{53: {"start", "firstkill", "end"}, 34: {"start", "end"}})
}

func TestWalkPlayer_StepOrderSemantics(t *testing.T) {
	p := newWalkPlayer(walkQuests())
	require.False(t, p.HasQuest("53-start"))

	p.grant("53-firstkill")
	require.False(t, p.HasQuest("53-start"), "a quest is entered only at its first step")

	p.grant("53-start")
	require.True(t, p.HasQuest("53-start"))
	require.False(t, p.HasQuest("53-firstkill"))

	p.grant("53-end")
	require.True(t, p.HasQuest("53-firstkill"), "a later step implies the earlier ones")
	require.Equal(t, "end", p.progress[53])

	p.grant("53-start")
	require.Equal(t, "end", p.progress[53], "a grant never moves back")
}

// A quest is entered only at "start", or at the only step of a one-step
// quest, as quests.IsTokenAfterOnPath allows: not at whatever step comes first.
func TestWalkPlayer_EntersOnlyAtStart(t *testing.T) {
	p := newWalkPlayer(stepQuests(map[int][]string{60: {"begin", "end"}, 61: {"end"}}))
	p.grant("60-begin")
	require.False(t, p.HasQuest("60-begin"), "a first step not named start cannot be entered")
	p.grant("61-end")
	require.True(t, p.HasQuest("61-end"), "a one-step quest is entered at its only step")
}

// The walk player reads Character's path rule with its own flags (#958): a
// side's step is granted only once the flag names that side, and a player on
// one side never holds the other side's steps.
func TestWalkPlayer_FollowsThePath(t *testing.T) {
	w := forkedWalkWorld()
	p := newWalkPlayer(map[int]*quests.Quest{54: w.Quests[0]})
	p.grant("54-start")
	p.grant("54-rhett")
	require.Equal(t, "start", p.progress[54], "no side's step before the flag names the side")

	p.flags["54-branch"] = "rhett"
	p.grant("54-sylara")
	require.Equal(t, "start", p.progress[54], "never the other side's step")
	p.grant("54-rhett")
	p.grant("54-samples")
	require.Equal(t, "samples", p.progress[54])
	require.True(t, p.HasQuest("54-rhett"))
	require.True(t, p.HasQuest("54-start"))
	require.False(t, p.HasQuest("54-totem"), "a Rhett player never holds Sylara's steps")
	require.False(t, p.HasQuest("54-sylara"))
}

func TestWalkPlayer_SetHeldAndClone(t *testing.T) {
	p := newWalkPlayer(walkQuests())
	p.hold("34-end")
	p.items[7] = true
	c := p.clone()
	c.grant("53-start")
	c.items[8] = true
	require.True(t, c.HasQuest("34-start"))
	require.False(t, p.HasQuest("53-start"), "a clone does not share progress")
	require.False(t, p.items[8], "a clone does not share items")
}

func TestWalkCtx_GrantGiveConsume(t *testing.T) {
	p := newWalkPlayer(walkQuests())
	ctx := &walkCtx{p: p}
	ctx.GrantQuest("53-start")
	require.NoError(t, ctx.GiveItem(5))
	ctx.ConsumeItem(5)
	ctx.SetQuestFlag("53-x", "y")
	require.True(t, p.HasQuest("53-start"))
	require.False(t, p.items[5])
	require.Equal(t, "y", p.GetQuestFlag("53-x"))
	var _ ActionContext = ctx // compile-time: walkCtx satisfies the interface
	var _ PlayerState = p
}

// The real bridge runs a sequence's on_complete actions through ExecuteAction
// (bridge.go QueueSequence), so a step granted inside one is reachable.
func TestWalkCtx_QueueSequenceRunsOnComplete(t *testing.T) {
	p := newWalkPlayer(walkQuests())
	ctx := &walkCtx{p: p}
	ctx.QueueSequence(SequenceDef{OnComplete: []ActionDef{{Grant: "53-start"}, {GiveItem: 9}}})
	require.True(t, p.HasQuest("53-start"))
	require.True(t, p.items[9])
}

func TestDialogueState_RoutesToThePlayer(t *testing.T) {
	p := newWalkPlayer(walkQuests())
	ps := p.dialogueState()
	ps.GiveQuest("53-start")
	require.True(t, ps.HasQuest("53-start"))
	require.True(t, ps.GiveItem(9))
	require.True(t, ps.HasItem(9))
	require.True(t, ps.RemoveItem(9))
	require.False(t, ps.HasItem(9))
	require.False(t, ps.HasHeard("9112.lore"), "a topic is heard only once marked")
	ps.MarkHeard("9112.lore")
	require.True(t, ps.HasHeard("9112.lore"))
	require.True(t, p.clone().heard["9112.lore"], "a clone keeps what was heard")
	require.True(t, ps.QuestOpen(53))
}
