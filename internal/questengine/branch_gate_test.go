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
