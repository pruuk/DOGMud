package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingCtx is a full mock action context that keeps count: progress per
// quest, as GameBridge keeps it per current step.
type countingCtx struct {
	*fullMockActionContext
	counts map[int]int
}

func newCountingCtx(p *fullMockPlayer) *countingCtx {
	return &countingCtx{fullMockActionContext: newFullMockActionContext(1, p), counts: map[int]int{}}
}

func (c *countingCtx) BumpStepCount(questId int) int {
	c.counts[questId]++
	return c.counts[questId]
}

func huntQuest() *QuestDef {
	return &QuestDef{
		QuestId: 53, Name: "Hunt",
		Steps: []QuestStep{{Id: "start"}, {Id: "end"}},
		Triggers: []TriggerDef{{
			Event: "mob_death", Mob: 9110, Count: 3,
			Conditions: Conditions{Has: []string{"53-start"}, Missing: []string{"53-end"}},
			Actions:    []ActionDef{{Grant: "53-end"}},
		}},
	}
}

func kill(e *Engine, mobId int, p *fullMockPlayer, ctx ActionContext) NotifyResult {
	return e.Notify("mob_death", EventDetails{UserId: 1, MobId: mobId, RoomId: 100}, p, ctx)
}

func TestCount_FiresOnTheNthMatch(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(huntQuest())
	p := newFullMockPlayer(100)
	p.quests["53-start"] = true
	ctx := newCountingCtx(p)

	r := kill(e, 9110, p, ctx)
	assert.True(t, r.Handled, "a counted match is a handled event")
	assert.False(t, p.HasQuest("53-end"))
	kill(e, 9110, p, ctx)
	assert.False(t, p.HasQuest("53-end"))
	kill(e, 9110, p, ctx)
	assert.True(t, p.HasQuest("53-end"), "the third kill fires")
	assert.Equal(t, 3, ctx.counts[53])
}

func TestCount_NonMatchingAndFailedConditionsDoNotCount(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(huntQuest())
	p := newFullMockPlayer(100)
	ctx := newCountingCtx(p)

	kill(e, 9110, p, ctx)
	assert.Equal(t, 0, ctx.counts[53], "conditions fail before the quest starts")
	p.quests["53-start"] = true
	r := kill(e, 9999, p, ctx)
	assert.False(t, r.Handled)
	assert.Equal(t, 0, ctx.counts[53], "another mob does not count")
}

// One event bumps a count once however many of its details match.
func TestCount_OneBumpPerEvent(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(huntQuest())
	p := newFullMockPlayer(100)
	p.quests["53-start"] = true
	ctx := newCountingCtx(p)
	d := EventDetails{UserId: 1, MobId: 9110, RoomId: 100}
	e.NotifyAll("mob_death", []EventDetails{d, d}, p, ctx)
	assert.Equal(t, 1, ctx.counts[53])
}

func TestCount_ContextWithoutCounterFiresAtOnce(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(huntQuest())
	p := newFullMockPlayer(100)
	p.quests["53-start"] = true
	kill(e, 9110, p, newFullMockActionContext(1, p))
	require.True(t, p.HasQuest("53-end"))
}

func TestCount_WalkCtxReachesAnyCount(t *testing.T) {
	var sc StepCounter = &walkCtx{}
	assert.GreaterOrEqual(t, sc.BumpStepCount(53), 1_000_000)
}
