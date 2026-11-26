package gmcp

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// A touch button press obeys the input rules a Char.Action.Try obeys (owner
// rulings 2026-11-25): it is a command from the client, so it counts against
// the player's pending inputs and is refused at the cap, and on the AI port
// it spends the per-round command budget.

// At the cap a press is not queued, and the refusal reaches the player as
// the one plain line every client gets.
func TestPressButton_IsRefusedAtTheCap(t *testing.T) {
	viewer, _ := pressFixture(t)
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = 2
	configs.SetConfigForTest(t, cfg)
	events.DrainQueuedByTypeForTest(`InputQueueChanged`)
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	require.True(t, pressButton(viewer, "m1"))
	require.True(t, pressButton(viewer, "m2"))
	require.Equal(t, 2, events.PendingInputs(viewer.UserId), "each press counts")
	events.DrainQueuedByTypeForTest(`InputQueueChanged`)

	require.False(t, pressButton(viewer, "m3"), "the press past the cap is refused")
	got := queuedInputs(viewer.UserId)
	require.Len(t, got, 2, "only the two that filled it")
	require.Equal(t, "north", got[0].InputText)
	require.Equal(t, "south", got[1].InputText)

	var refused []events.InputQueueChanged
	for _, e := range events.DrainQueuedByTypeForTest(`InputQueueChanged`) {
		if c := e.(events.InputQueueChanged); c.UserId == viewer.UserId && c.Refused {
			refused = append(refused, c)
		}
	}
	require.Len(t, refused, 1, "the refusal is reported")
	events.DrainQueuedMessagesForTest(viewer.UserId)
	inputQueueHandler(refused[0])
	lines := events.DrainQueuedMessagesForTest(viewer.UserId)
	require.Len(t, lines, 1, "a refused press is not silent")
	require.Contains(t, lines[0], refusal.InputQueueFullLine)
}

// pressOnConn sends one Char.Button.Press through handleCharOp and returns
// how many commands it queued.
func pressOnConn(t *testing.T, connId uint64, id string) int {
	t.Helper()
	events.DrainQueuedByTypeForTest(`Input`)
	(&GMCPModule{}).handleCharOp(GMCPCharOp{
		ConnectionId: connId,
		Command:      `Char.Button.Press`,
		Payload:      []byte(`{"id":"` + id + `"}`),
	})
	return len(events.DrainQueuedByTypeForTest(`Input`))
}

// On the AI port a press spends the per-round budget: the fourth command in
// a round is dropped with the typed path's line, a press and a Try share the
// one counter, and a new round restores it.
func TestButtonPress_AIPortIsHeldToThePerRoundLimit(t *testing.T) {
	cd, screen := aiLimitFixture(t, 9872, connections.ConnAI)
	id := cd.ConnectionId()

	for i := 1; i <= 3; i++ {
		require.Equal(t, 1, pressOnConn(t, id, "m1"), "press %d is within the budget and runs", i)
	}
	require.Equal(t, 0, pressOnConn(t, id, "m1"), "the fourth press in a round must not run")
	require.Eventually(t, func() bool {
		return strings.Contains(screen(), strings.TrimRight(connections.AIRateLimitLine(3), "\r\n"))
	}, 2*time.Second, 5*time.Millisecond, "the refusal sends the typed path's drop line")

	util.SetRoundCountForTest(501)
	_, queued := tryAction(t, id, 1)
	require.Equal(t, 1, queued, "a new round restores the budget")
	_, queued = tryAction(t, id, 2)
	require.Equal(t, 1, queued)
	require.Equal(t, 1, pressOnConn(t, id, "m2"), "the third command of the round runs")
	require.Equal(t, 0, pressOnConn(t, id, "m2"), "a press and a Try share the budget")
}

// A connection on a normal port has no per-round budget for presses.
func TestButtonPress_NormalPortIsUnaffected(t *testing.T) {
	cd, screen := aiLimitFixture(t, 9873, connections.ConnHuman)
	for i := 1; i <= 6; i++ {
		require.Equal(t, 1, pressOnConn(t, cd.ConnectionId(), "m1"), "press %d on a normal port runs", i)
	}
	require.NotContains(t, screen(), "AI rate limit")
}
