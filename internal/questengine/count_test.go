package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingCtx is a full mock action context that keeps count: progress by
// the key the engine passes, as GameBridge does. It leaves the step check to
// the bridge (TestGameBridge_BumpStepCount*).
type countingCtx struct {
	*fullMockActionContext
	counts map[string]int
}

func newCountingCtx(p *fullMockPlayer) *countingCtx {
	return &countingCtx{fullMockActionContext: newFullMockActionContext(1, p), counts: map[string]int{}}
}

func (c *countingCtx) BumpStepCount(questId int, step, key string) (int, bool) {
	c.counts[key]++
	return c.counts[key], true
}

// huntKey is the count key of hunt's one trigger.
const huntKey = "53-start-t0"

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
	assert.Equal(t, 3, ctx.counts[huntKey])
}

func TestCount_NonMatchingAndFailedConditionsDoNotCount(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(huntQuest())
	p := newFullMockPlayer(100)
	ctx := newCountingCtx(p)

	kill(e, 9110, p, ctx)
	assert.Equal(t, 0, ctx.counts[huntKey], "conditions fail before the quest starts")
	p.quests["53-start"] = true
	r := kill(e, 9999, p, ctx)
	assert.False(t, r.Handled)
	assert.Equal(t, 0, ctx.counts[huntKey], "another mob does not count")
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
	assert.Equal(t, 1, ctx.counts[huntKey])
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
	n, ok := sc.BumpStepCount(53, "start", huntKey)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, n, 1_000_000)
}

// Two count triggers on one step keep their own counts: two kills of mob 1
// and one of mob 2 do not reach mob 2's three (the reviewer's repro).
func TestCount_TwoTriggersOnOneStepKeepTheirOwnCounts(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{
		QuestId: 53, Name: "Hunt",
		Steps: []QuestStep{{Id: "start"}, {Id: "end"}},
		Triggers: []TriggerDef{
			{
				Event: "mob_death", Mob: 1, Count: 5,
				Conditions: Conditions{Has: []string{"53-start"}, Missing: []string{"53-end"}},
				Actions:    []ActionDef{{Grant: "53-end"}},
			},
			{
				Event: "mob_death", Mob: 2, Count: 3,
				Conditions: Conditions{Has: []string{"53-start"}, Missing: []string{"53-end"}},
				Actions:    []ActionDef{{Grant: "53-end"}},
			},
		},
	})
	p := newFullMockPlayer(100)
	p.quests["53-start"] = true
	ctx := newCountingCtx(p)

	kill(e, 1, p, ctx)
	kill(e, 1, p, ctx)
	kill(e, 2, p, ctx)
	assert.False(t, p.HasQuest("53-end"), "mob 2's count of three is not reached by kills of mob 1")
	assert.Equal(t, 2, ctx.counts["53-start-t0"])
	assert.Equal(t, 1, ctx.counts["53-start-t1"])
}

// A context that refuses the bump (the player is not on the trigger's step)
// neither fires the trigger nor marks the event handled.
type refusingCtx struct{ *fullMockActionContext }

func (refusingCtx) BumpStepCount(int, string, string) (int, bool) { return 0, false }

func TestCount_RefusedBumpNeitherFiresNorHandles(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(huntQuest())
	p := newFullMockPlayer(100)
	p.quests["53-start"] = true
	r := kill(e, 9110, p, refusingCtx{newFullMockActionContext(1, p)})
	assert.False(t, r.Handled)
	assert.False(t, p.HasQuest("53-end"))
}
