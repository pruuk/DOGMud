package main

import (
	"io"
	"net"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// macroQueued runs a 4-command macro for a user on a live connection of the
// given type, after the line naming it has spent one command of the round
// (as main.go's read loop does for every typed line), and returns how many
// of the macro's commands were queued.
func macroQueued(t *testing.T, userId int, connType connections.ConnType) int {
	t.Helper()

	cfg := configs.GetConfig()
	cfg.Network.AICommandsPerRound = 3
	configs.SetConfigForTest(t, cfg)

	util.SetRoundCountForTest(700)
	t.Cleanup(util.ResetRoundCountForTest)

	server, client := net.Pipe()
	cd := connections.Add(server, nil, connType)
	id := cd.ConnectionId()
	t.Cleanup(func() { _ = connections.Remove(id); _ = client.Close() })
	go func() { _, _ = io.Copy(io.Discard, client) }()

	u := users.NewTestUser(userId, "macrouser", "Macrouser", id)
	u.Macros = map[string]string{`=1`: `look;look;look;look`}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))

	events.DrainQueuedByTypeForTest(`Input`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`Input`) })

	require.False(t, cd.AIRateLimited(), "the line naming the macro fits the budget")
	(&World{}).processInput(userId, `=1`, 0, 0)

	return len(events.DrainQueuedByTypeForTest(`Input`))
}

// A macro on the AI port spends one command per piece from the same per-round
// budget: with a budget of 3, a 4-command macro queues only 3 commands.
func TestProcessInput_MacroOnTheAIPortSpendsTheBudgetPerCommand(t *testing.T) {
	require.Equal(t, 3, macroQueued(t, 9841, connections.ConnAI))
}

// A macro on a normal port queues every command, as before.
func TestProcessInput_MacroOnANormalPortIsUnaffected(t *testing.T) {
	require.Equal(t, 4, macroQueued(t, 9842, connections.ConnHuman))
}
