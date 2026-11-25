package gmcp

import (
	"bytes"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// aiLimitFixture is a live connection of the given type with a logged-in user
// standing in a room, the AI per-round budget pinned to 3, and the round set.
// screen returns everything the connection has been sent so far.
func aiLimitFixture(t *testing.T, userId int, connType connections.ConnType) (cd *connections.ConnectionDetails, screen func() string) {
	t.Helper()

	cfg := configs.GetConfig()
	cfg.Network.AICommandsPerRound = 3
	configs.SetConfigForTest(t, cfg)

	util.SetRoundCountForTest(500)
	t.Cleanup(util.ResetRoundCountForTest)

	server, client := net.Pipe()
	cd = connections.Add(server, nil, connType)
	id := cd.ConnectionId()
	t.Cleanup(func() { _ = connections.Remove(id); _ = client.Close() })

	var mu sync.Mutex
	var buf bytes.Buffer
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := client.Read(chunk)
			mu.Lock()
			buf.Write(chunk[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	screen = func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}

	u := menuTestUser(t, userId, id, true)
	t.Cleanup(keywords.SeedKeywordsForTest())
	roomId := userId
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{roomId: {RoomId: roomId}},
		map[string]*rooms.ZoneConfig{}))
	u.Character.RoomId = roomId

	return cd, screen
}

// tryAction runs one Char.Action.Try through handleCharOp and returns the
// Char.Action.Result status and how many commands it queued.
func tryAction(t *testing.T, connId uint64, actionId int) (status string, queued int) {
	t.Helper()
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	events.DrainQueuedByTypeForTest(`Input`)

	(&GMCPModule{}).handleCharOp(GMCPCharOp{
		ConnectionId: connId,
		Command:      `Char.Action.Try`,
		Payload:      []byte(`{"id":` + strconv.Itoa(actionId) +`,"cmd":"look"}`),
	})

	for _, o := range drainGMCPOut(t) {
		if o.Module == `Char.Action.Result` {
			r := o.Payload.(GMCPAction_Result)
			require.Equal(t, actionId, r.Id)
			status = r.Status
		}
	}
	require.NotEmpty(t, status, "every Try must be answered")
	return status, len(events.DrainQueuedByTypeForTest(`Input`))
}

// On the AI port a Char.Action.Try spends the per-round command budget: the
// fourth Try in a round is dropped, answered "rejected", and the connection
// gets the same line a dropped typed command gets. The next round the budget
// is whole again.
func TestActionTry_AIPortIsHeldToThePerRoundLimit(t *testing.T) {
	cd, screen := aiLimitFixture(t, 9831, connections.ConnAI)
	id := cd.ConnectionId()

	for i := 1; i <= 3; i++ {
		_, queued := tryAction(t, id, i)
		require.Equal(t, 1, queued, "Try %d is within the budget and runs", i)
	}

	status, queued := tryAction(t, id, 4)
	require.Equal(t, 0, queued, "the fourth Try in a round must not run")
	require.Equal(t, "rejected", status)
	require.Eventually(t, func() bool {
		// Output turns each \n into \r\n, so compare the line's text.
		return strings.Contains(screen(), strings.TrimRight(connections.AIRateLimitLine(3), "\r\n"))
	}, 2*time.Second, 5*time.Millisecond, "the refusal sends the typed path's drop line")

	util.SetRoundCountForTest(501)
	_, queued = tryAction(t, id, 5)
	require.Equal(t, 1, queued, "a new round restores the budget")
}

// Typed lines and Try share one budget: two typed lines and two Try in one
// round let only three commands through.
func TestActionTry_SharesTheBudgetWithTypedLines(t *testing.T) {
	cd, _ := aiLimitFixture(t, 9832, connections.ConnAI)
	id := cd.ConnectionId()

	// The typed path (main.go) asks AIRateLimited once per submitted line.
	require.False(t, cd.AIRateLimited(), "typed line 1")
	require.False(t, cd.AIRateLimited(), "typed line 2")

	_, queued := tryAction(t, id, 1)
	require.Equal(t, 1, queued, "the third command of the round runs")

	status, queued := tryAction(t, id, 2)
	require.Equal(t, 0, queued, "the fourth command of the round is dropped")
	require.Equal(t, "rejected", status)
}

// A connection on a normal port has no per-round budget at all.
func TestActionTry_NormalPortIsUnaffected(t *testing.T) {
	cd, screen := aiLimitFixture(t, 9833, connections.ConnHuman)
	id := cd.ConnectionId()

	for i := 1; i <= 6; i++ {
		_, queued := tryAction(t, id, i)
		require.Equal(t, 1, queued, "Try %d on a normal port runs", i)
	}
	for i := 0; i < 6; i++ {
		require.False(t, cd.AIRateLimited(), "a typed line on a normal port is never limited")
	}
	require.NotContains(t, screen(), "AI rate limit")
}
