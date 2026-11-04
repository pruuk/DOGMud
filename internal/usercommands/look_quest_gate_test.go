package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/stretchr/testify/require"
)

// A room_gate refuses a look through the exit in the gate's own words, the line
// the step would read, and a scan reads the exit as locked. A player the gate
// allows sees as usual. seedDarknessGateRoom stands the player in a room lit
// enough to see out, with the Town Square to the south.
func TestLookAndScan_QuestGatedExit(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 90)
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999642,
		Name:    "Look Gate Quest",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:      "room_gate",
			Room:       1,
			Conditions: questengine.Conditions{Missing: []string{"999642-start"}},
			Actions:    []questengine.ActionDef{{SendText: gateTestLine}},
		}},
	})
	look := func() (bool, error) { return Look("south", user, room, 0) }
	scan := func() (bool, error) { return Scan("", user, room, 0) }

	out := runGate(t, user, look)
	require.Contains(t, out, gateTestLine, "the gate says why")
	require.NotContains(t, out, "You peer", "no view through the exit")
	require.NotContains(t, out, "Town Square")

	out = runGate(t, user, scan)
	require.Contains(t, out, `south</ansi>: the exit is locked`)
	require.NotContains(t, out, "Town Square")
	require.NotContains(t, out, "Skeleton")
	require.NotContains(t, out, gateTestLine, "a scan narrates no gate")

	user.Character.QuestProgress = map[int]string{999642: "start"}
	t.Cleanup(func() { delete(user.Character.QuestProgress, 999642) })

	out = runGate(t, user, look)
	require.Contains(t, out, "You peer", "the gate allows this player")
	require.NotContains(t, out, gateTestLine)

	out = runGate(t, user, scan)
	require.NotContains(t, out, `the exit is locked`)
	require.Contains(t, out, "Skeleton", "and shows who is there")
}
