package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mapPlayer is a full mock player with a map (MapReader).
type mapPlayer struct {
	*fullMockPlayer
	mapped map[int]bool
}

func newMapPlayer(mapped ...int) *mapPlayer {
	p := &mapPlayer{fullMockPlayer: newFullMockPlayer(100), mapped: map[int]bool{}}
	for _, id := range mapped {
		p.mapped[id] = true
	}
	return p
}

func (p *mapPlayer) MappedCount(roomIds []int) int {
	n := 0
	for _, id := range roomIds {
		if p.mapped[id] {
			n++
		}
	}
	return n
}

// exploreQuest: "begin" at mob 79 moves 340 to explore; exploring the grove
// (distinct) ends it.
func exploreQuest(distinct, line string) *QuestDef {
	return &QuestDef{
		QuestId: 340, Name: "Know the Grove",
		Steps: []QuestStep{{Id: "start"}, {Id: "explore"}, {Id: "end"}},
		Triggers: []TriggerDef{
			{
				Event: "command", Command: "begin",
				Conditions: Conditions{Has: []string{"340-start"}, Missing: []string{"340-explore"}},
				Actions:    []ActionDef{{Grant: "340-explore"}},
			},
			{
				Event: "room_enter", Area: "grove", Distinct: distinct, ExploredLine: line,
				Conditions: Conditions{Has: []string{"340-explore"}, Missing: []string{"340-end"}},
				Actions:    []ActionDef{{Grant: "340-end"}},
			},
		},
	}
}

func exploring(mapped ...int) (*mapPlayer, *fullMockActionContext) {
	p := newMapPlayer(mapped...)
	p.quests["340-start"] = true
	p.quests["340-explore"] = true
	return p, newFullMockActionContext(1, p.fullMockPlayer)
}

func TestExplore_NotMatchedOnRoomEnter(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(exploreQuest("all", ""))
	p, ctx := exploring(5311, 5312, 5313)
	r := e.Notify("room_enter", EventDetails{UserId: 1, RoomId: 5313}, p, ctx)
	assert.False(t, r.Handled)
	assert.False(t, p.HasQuest("340-end"), "an explore step is read from the map, not room entry")
}

func TestExplore_CheckFiresWhenTheAreaIsMapped(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(exploreQuest("all", ""))
	p, ctx := exploring(5311, 5312)
	assert.False(t, e.CheckExplore(5312, p, ctx).Handled, "two of three")
	assert.False(t, p.HasQuest("340-end"))

	p.mapped[5313] = true
	assert.True(t, e.CheckExplore(5313, p, ctx).Handled, "three of three")
	assert.True(t, p.HasQuest("340-end"))
	assert.Empty(t, ctx.sentTexts, "the explored line is for step start only")
}

func TestExplore_CheckIgnoresRoomsOutsideTheArea(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(exploreQuest("all", ""))
	p, ctx := exploring(5311, 5312, 5313)
	assert.False(t, e.CheckExplore(100, p, ctx).Handled)
}

func TestExplore_DistinctN(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(exploreQuest("2", ""))
	p, ctx := exploring(5311)
	assert.False(t, e.CheckExplore(5311, p, ctx).Handled)
	p.mapped[5313] = true
	assert.True(t, e.CheckExplore(5313, p, ctx).Handled)
	assert.True(t, p.HasQuest("340-end"))
}

func TestExplore_ConditionsGate(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(exploreQuest("all", ""))
	p, ctx := exploring(5311, 5312, 5313)
	delete(p.quests, "340-explore")
	assert.False(t, e.CheckExplore(5313, p, ctx).Handled, "not on the step yet")
}

func TestExplore_NoMapReaderNeverFires(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(exploreQuest("all", ""))
	p := newFullMockPlayer(100)
	p.quests["340-start"], p.quests["340-explore"] = true, true
	assert.False(t, e.CheckExplore(5313, p, newFullMockActionContext(1, p)).Handled)
}

// At step start an area already on the map fires the step at once, with the
// explored line: the area's name by default, the trigger's own when set.
func TestExplore_StepStartInTheGrantChain(t *testing.T) {
	installGrove(t)
	for _, c := range []struct{ distinct, line, want string }{
		{"all", "", "You already know every corner of the grove."},
		{"2", "", "You have already seen enough of the grove."},
		{"all", "You know these trees.", "You know these trees."},
	} {
		e := NewEngine()
		e.RegisterQuest(exploreQuest(c.distinct, c.line))
		p := newMapPlayer(5311, 5312, 5313)
		p.quests["340-start"] = true
		ctx := newFullMockActionContext(1, p.fullMockPlayer)
		e.Notify("command", EventDetails{UserId: 1, Command: "begin", TargetKind: "none"}, p, ctx)
		assert.True(t, p.HasQuest("340-end"), "the explore step fires as it starts")
		assert.Equal(t, []string{c.want}, ctx.sentTexts)
	}
}

// A step granted outside the engine (dialogue, HandleQuestUpdate's bridge)
// raises quest_granted, and that is a step start too.
func TestExplore_StepStartOnAnOutsideGrant(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(exploreQuest("all", ""))
	p, ctx := exploring(5311, 5312, 5313)
	r := e.Notify("quest_granted", EventDetails{UserId: 1, QuestToken: "340-explore"}, p, ctx)
	assert.True(t, r.Handled)
	assert.True(t, p.HasQuest("340-end"))
}

func TestExplore_StepStartWithAnUnmappedArea(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(exploreQuest("all", ""))
	p, ctx := exploring(5311)
	e.Notify("quest_granted", EventDetails{UserId: 1, QuestToken: "340-explore"}, p, ctx)
	assert.False(t, p.HasQuest("340-end"))
	assert.Empty(t, ctx.sentTexts)
}

// Explore steps in a row, each already explored, chain through one another
// at step start; the EvalGuard bounds the chain (each trigger fires once,
// and depth is limited).
func TestExplore_ChainedStepStartsAreBounded(t *testing.T) {
	installGrove(t)
	q := &QuestDef{
		QuestId: 341, Name: "Again and Again",
		Steps: []QuestStep{{Id: "start"}, {Id: "a"}, {Id: "b"}, {Id: "end"}},
	}
	for _, s := range [][2]string{{"start", "a"}, {"a", "b"}, {"b", "end"}} {
		q.Triggers = append(q.Triggers, TriggerDef{
			Event: "room_enter", Area: "grove", Distinct: "all",
			Conditions: Conditions{Has: []string{"341-" + s[0]}, Missing: []string{"341-" + s[1]}},
			Actions:    []ActionDef{{Grant: "341-" + s[1]}},
		})
	}
	e := NewEngine()
	e.RegisterQuest(q)
	p := newMapPlayer(5311, 5312, 5313)
	ctx := newFullMockActionContext(1, p.fullMockPlayer)
	ctx.GrantQuest("341-start")
	e.Notify("quest_granted", EventDetails{UserId: 1, QuestToken: "341-start"}, p, ctx)
	require.True(t, p.HasQuest("341-end"), "every step was explored already")
	assert.Len(t, ctx.sentTexts, 3)
}

// The walk proves an explore step reachable: its player has mapped everything.
func TestExplore_WalkPlayerReadsAFullMap(t *testing.T) {
	var mr MapReader = newWalkPlayer(nil)
	assert.Equal(t, 3, mr.MappedCount([]int{5311, 5312, 5313}))
}

func TestWalk_ExploreStepFinishes(t *testing.T) {
	installGrove(t)
	w := walkWorld()
	td := &w.Quests[0].Triggers[0]
	td.Event, td.Mob, td.Area, td.Distinct = "room_enter", 0, "grove", "all"
	require.Empty(t, Walk(w, questloop.Sources{}))
}
