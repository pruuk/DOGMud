package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// menuTestUser seeds one user. When gmcpOn is set the user has negotiated GMCP.
func menuTestUser(t *testing.T, userId int, connId uint64, gmcpOn bool) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(userId, "menuuser", "Menuuser", connId)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))
	if gmcpOn {
		gmcpModule.cache.Add(connId, GMCPSettings{GMCPAccepted: true, HelloReceived: true})
		t.Cleanup(func() { gmcpModule.cache.Remove(connId) })
	}
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	events.DrainQueuedByTypeForTest(`MenuChanged`)
	events.DrainQueuedByTypeForTest(`Input`)
	t.Cleanup(func() {
		events.DrainQueuedByTypeForTest(`GMCPOut`)
		events.DrainQueuedByTypeForTest(`MenuChanged`)
		events.DrainQueuedByTypeForTest(`Input`)
	})
	return u
}

func drainGMCPOut(t *testing.T) []GMCPOut {
	t.Helper()
	var out []GMCPOut
	for _, e := range events.DrainQueuedByTypeForTest(`GMCPOut`) {
		out = append(out, e.(GMCPOut))
	}
	return out
}

func TestMenuChanged_PushesCharMenuToAGMCPClient(t *testing.T) {
	u := menuTestUser(t, 9821, 98821, true)

	for _, open := range []bool{true, false} {
		menuChangedHandler(events.MenuChanged{UserId: u.UserId, Open: open})
		sent := drainGMCPOut(t)
		require.Len(t, sent, 1)
		require.Equal(t, u.UserId, sent[0].UserId)
		require.Equal(t, `Char.Menu`, sent[0].Module)
		require.Equal(t, map[string]bool{"open": open}, sent[0].Payload)
	}
}

func TestMenuChanged_SendsNothingWithoutGMCPOrUser(t *testing.T) {
	u := menuTestUser(t, 9822, 98822, false)

	menuChangedHandler(events.MenuChanged{UserId: u.UserId, Open: true})
	require.Empty(t, drainGMCPOut(t), "a client without GMCP is never pushed")

	menuChangedHandler(events.MenuChanged{UserId: 0, Open: true})
	require.Empty(t, drainGMCPOut(t), "UserId 0 is ignored")

	menuChangedHandler(events.MenuChanged{UserId: 9899, Open: true})
	require.Empty(t, drainGMCPOut(t), "an unknown user is ignored")
}

// While a prompt is open a queued Char.Action.Try is answered "deferred" and
// the command is not run; with nothing open the same call is not deferred.
func TestActionTry_DefersWhileAPromptIsOpen(t *testing.T) {
	u := menuTestUser(t, 9823, 98823, true)
	try := func() {
		(&GMCPModule{}).handleCharOp(GMCPCharOp{
			ConnectionId: 98823,
			Command:      `Char.Action.Try`,
			Payload:      []byte(`{"id":7,"cmd":"look"}`),
		})
	}

	u.StartPrompt(`test`, ``)
	require.True(t, u.MenuOrPromptOpen())
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	events.DrainQueuedByTypeForTest(`Input`)

	try()
	var result *GMCPAction_Result
	for _, o := range drainGMCPOut(t) {
		if o.Module == `Char.Action.Result` {
			r := o.Payload.(GMCPAction_Result)
			result = &r
		}
	}
	require.NotNil(t, result, "the Try must be answered")
	require.Equal(t, 7, result.Id)
	require.Equal(t, "deferred", result.Status)
	require.Equal(t, "menu open", result.Reason)
	require.Empty(t, events.DrainQueuedByTypeForTest(`Input`), "the command must not run")

	// With nothing open the same call is not deferred. A bare test user has no
	// room, so readiness may panic; the point is only that the guard is passed.
	u.ClearPrompt()
	require.False(t, u.MenuOrPromptOpen())
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	func() {
		defer func() { _ = recover() }()
		try()
	}()
	for _, o := range drainGMCPOut(t) {
		if o.Module == `Char.Action.Result` {
			require.NotEqual(t, "deferred", o.Payload.(GMCPAction_Result).Status)
		}
	}
}

// A reconnect reuses the user record, so an open prompt survives it. The fresh
// client starts unpaused and must be told: once at spawn, once next round for a
// web client whose GMCP hello raced the login burst.
func TestPlayerSpawn_ResendsAnOpenMenuToAReturningClient(t *testing.T) {
	u := menuTestUser(t, 9824, 98824, true)
	t.Cleanup(func() { menuSpawnPending = map[int]struct{}{} })

	menuPlayerSpawnHandler(events.PlayerSpawn{UserId: u.UserId})
	require.Empty(t, drainGMCPOut(t), "nothing open, nothing to resend")
	menuNewRoundHandler(events.NewRound{})
	require.Empty(t, drainGMCPOut(t))

	u.StartPrompt(`test`, ``)
	events.DrainQueuedByTypeForTest(`GMCPOut`)

	menuPlayerSpawnHandler(events.PlayerSpawn{UserId: u.UserId})
	sent := drainGMCPOut(t)
	require.Len(t, sent, 1)
	require.Equal(t, `Char.Menu`, sent[0].Module)
	require.Equal(t, map[string]bool{"open": true}, sent[0].Payload)

	menuNewRoundHandler(events.NewRound{})
	require.Len(t, drainGMCPOut(t), 1, "one re-push next round")
	menuNewRoundHandler(events.NewRound{})
	require.Empty(t, drainGMCPOut(t), "the re-push happens once")
}
