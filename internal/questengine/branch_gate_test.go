package questengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// seedBranchEntry registers quest 11's shape (#958) with one branch-entry
// trigger: entering room 600 at start speaks, hands over an item and grants
// Rhett's first step. It carries no flag condition; the engine guards the side.
func seedBranchEntry() *Engine {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{
		QuestId: 97511,
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
		Triggers: []TriggerDef{{
			Event:      "room_enter",
			Room:       600,
			Conditions: Conditions{Has: []string{"97511-start"}, Missing: []string{"97511-rhett"}},
			Actions: []ActionDef{
				{SendText: "Rhett waves you over and presses a sample bag into your hands."},
				{GiveItem: 4242},
				{Grant: "97511-rhett"},
			},
		}},
	})
	return e
}

func branchPlayer(flag string, held ...string) *fullMockPlayer {
	p := newFullMockPlayer(600)
	for _, h := range held {
		p.quests[h] = true
	}
	if flag != "" {
		p.flags["97511-branch"] = flag
	}
	return p
}

// A trigger that would grant a step off the player's side does nothing at all:
// no text, no item, no grant, and the command is not taken.
func TestBranchGate_OffPathTriggerDoesNothing(t *testing.T) {
	cases := []struct {
		name   string
		player *fullMockPlayer
	}{
		{"sylara side at start", branchPlayer("sylara", "97511-start")},
		{"no side chosen", branchPlayer("", "97511-start")},
		{"on sylara's step with a rhett flag", branchPlayer("rhett", "97511-start", "97511-sylara")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := seedBranchEntry()
			ctx := &recordingCtx{}
			d := EventDetails{UserId: 1, RoomId: 600}

			require.False(t, e.WouldFire("room_enter", d, c.player), "the talk menu reads the same gate")
			res := e.Notify("room_enter", d, c.player, ctx)
			require.False(t, res.Handled)
			require.Empty(t, ctx.textsSent)
			require.Empty(t, ctx.gaveItem)
			require.Empty(t, ctx.granted)
		})
	}
}

// forkedSteps is the branched quest's steps, shared by the tests below.
func forkedSteps() []QuestStep {
	return []QuestStep{
		{Id: "start"},
		{Id: "sylara", Branch: "sylara"},
		{Id: "totem", Branch: "sylara"},
		{Id: "rhett", Branch: "rhett"},
		{Id: "samples", Branch: "rhett"},
		{Id: "end"},
	}
}

func forkedQuest(triggers ...TriggerDef) *QuestDef {
	return &QuestDef{
		QuestId: 97511, Name: "Forked",
		Flags:    []QuestFlagDef{{Key: "branch", Values: []string{"sylara", "rhett"}}},
		Steps:    forkedSteps(),
		Triggers: triggers,
	}
}

// A trigger that sets the branch flag and then grants that side's step picks
// the side itself: the flag lands before the grant (set_flag then grant is
// how quest 18 records an outcome), so the gate reads the trigger's own flag
// for a player who holds no branched step yet.
func TestBranchGate_SetFlagEarlierInTheTriggerChoosesTheSide(t *testing.T) {
	choose := func(actions ...ActionDef) *Engine {
		e := NewEngine()
		e.RegisterQuest(forkedQuest(TriggerDef{
			Event: "room_enter", Room: 600,
			Conditions: Conditions{Has: []string{"97511-start"}, Missing: []string{"97511-rhett"}},
			Actions:    actions,
		}))
		return e
	}
	setRhett := ActionDef{SetFlag: &QuestFlagAction{Key: "97511-branch", Value: "rhett"}}
	setSylara := ActionDef{SetFlag: &QuestFlagAction{Key: "97511-branch", Value: "sylara"}}
	grantRhett := ActionDef{Grant: "97511-rhett"}
	d := EventDetails{UserId: 1, RoomId: 600}

	cases := []struct {
		name    string
		flag    string
		actions []ActionDef
		fires   bool
	}{
		{"no flag, set rhett then grant rhett", "", []ActionDef{setRhett, grantRhett}, true},
		{"sylara flag, set rhett then grant rhett", "sylara", []ActionDef{setRhett, grantRhett}, true},
		{"no flag, set sylara then grant rhett", "", []ActionDef{setSylara, grantRhett}, false},
		{"no flag, grant rhett then set rhett", "", []ActionDef{grantRhett, setRhett}, false},
		{"no flag, the last set before the grant wins", "", []ActionDef{setSylara, setRhett, grantRhett}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := choose(c.actions...)
			p := branchPlayer(c.flag, "97511-start")
			require.Equal(t, c.fires, e.WouldFire("room_enter", d, p))
			ctx := &recordingCtx{}
			require.Equal(t, c.fires, e.Notify("room_enter", d, p, ctx).Handled)
			if c.fires {
				require.Equal(t, []string{"97511-rhett"}, ctx.granted)
			} else {
				require.Empty(t, ctx.granted)
				require.Empty(t, ctx.flagsSet, "a skipped trigger sets no flag either")
			}
		})
	}

	// A player already on a side is not moved by a trigger's own flag.
	e := choose(setRhett, grantRhett)
	p := branchPlayer("", "97511-start", "97511-sylara")
	require.False(t, e.WouldFire("room_enter", d, p))
}

// Another quest's trigger granting a branched step is gated the same way.
func TestBranchGate_CrossQuestGrant(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(forkedQuest())
	e.RegisterQuest(&QuestDef{
		QuestId: 97512, Name: "Errand",
		Steps: []QuestStep{{Id: "start"}, {Id: "end"}},
		Triggers: []TriggerDef{{
			Event: "room_enter", Room: 600,
			Actions: []ActionDef{{SendText: "A runner finds you."}, {Grant: "97511-rhett"}},
		}},
	})
	d := EventDetails{UserId: 1, RoomId: 600}

	ctx := &recordingCtx{}
	require.False(t, e.Notify("room_enter", d, branchPlayer("sylara", "97511-start"), ctx).Handled)
	require.Empty(t, ctx.textsSent)

	ctx = &recordingCtx{}
	require.True(t, e.Notify("room_enter", d, branchPlayer("rhett", "97511-start"), ctx).Handled)
	require.Equal(t, []string{"97511-rhett"}, ctx.granted)
}

// The explore checks (CheckExplore on a newly mapped room, and the step-start
// check in the grant chain) read the same gate.
func TestBranchGate_ExplorePaths(t *testing.T) {
	installGrove(t)
	explore := TriggerDef{
		Event: "room_enter", Area: "grove", Distinct: "all",
		Conditions: Conditions{Has: []string{"97511-start"}, Missing: []string{"97511-rhett"}},
		Actions:    []ActionDef{{SendText: "You know the grove."}, {Grant: "97511-rhett"}},
	}
	for _, c := range []struct {
		flag  string
		fires bool
	}{{"sylara", false}, {"rhett", true}} {
		t.Run(c.flag, func(t *testing.T) {
			e := NewEngine()
			e.RegisterQuest(forkedQuest(explore))
			p := newMapPlayer(5311, 5312, 5313)
			p.quests["97511-start"] = true
			p.flags["97511-branch"] = c.flag
			ctx := newFullMockActionContext(1, p.fullMockPlayer)
			require.Equal(t, c.fires, e.CheckExplore(5313, p, ctx).Handled, "CheckExplore")
			require.Equal(t, c.fires, p.HasQuest("97511-rhett"))

			e = NewEngine()
			e.RegisterQuest(forkedQuest(explore))
			p = newMapPlayer(5311, 5312, 5313)
			p.quests["97511-start"] = true
			p.flags["97511-branch"] = c.flag
			ctx = newFullMockActionContext(1, p.fullMockPlayer)
			e.Notify("quest_granted", EventDetails{UserId: 1, QuestToken: "97511-start"}, p, ctx)
			require.Equal(t, c.fires, p.HasQuest("97511-rhett"), "step start")
			if !c.fires {
				require.Empty(t, ctx.sentTexts)
			}
		})
	}
}

// The held-item check at step start reads the same gate.
func TestBranchGate_HeldAtStepStart(t *testing.T) {
	held := TriggerDef{
		Event: "item_gain", Item: 77,
		Conditions: Conditions{Has: []string{"97511-start"}, Missing: []string{"97511-rhett"}},
		Actions:    []ActionDef{{Grant: "97511-rhett"}},
	}
	for _, c := range []struct {
		flag  string
		fires bool
	}{{"sylara", false}, {"rhett", true}} {
		t.Run(c.flag, func(t *testing.T) {
			e := NewEngine()
			e.RegisterQuest(forkedQuest(held))
			p := branchPlayer(c.flag, "97511-start")
			p.items[77] = true
			ctx := newFullMockActionContext(1, p)
			e.Notify("quest_granted", EventDetails{UserId: 1, QuestToken: "97511-start"}, p, ctx)
			require.Equal(t, c.fires, p.HasQuest("97511-rhett"))
			if !c.fires {
				require.Empty(t, ctx.sentTexts, "no held line for the other side")
			}
		})
	}
}

func TestBranchGate_OnPathTriggerFires(t *testing.T) {
	e := seedBranchEntry()
	ctx := &recordingCtx{}
	d := EventDetails{UserId: 1, RoomId: 600}
	p := branchPlayer("rhett", "97511-start")

	require.True(t, e.WouldFire("room_enter", d, p))
	res := e.Notify("room_enter", d, p, ctx)
	require.True(t, res.Handled)
	require.Len(t, ctx.textsSent, 1)
	require.Equal(t, []int{4242}, ctx.gaveItem)
	require.Equal(t, []string{"97511-rhett"}, ctx.granted)
}
