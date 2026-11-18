package inputhandlers

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/auththrottle"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/stretchr/testify/require"
)

// kickStep is the login step that asks whether to kick a session already
// connected.
func kickStep(t *testing.T) *PromptStep {
	t.Helper()
	for _, step := range loginSteps() {
		if step.ID == `kickuser` {
			return step
		}
	}
	t.Fatal("no kickuser step")
	return nil
}

// A locked-out source is never asked the kick question: asking it would tell
// the source its password guess was right.
func TestKickQuestion_NotAskedWhileThrottled(t *testing.T) {
	step := kickStep(t)
	require.False(t, step.Condition(map[string]string{
		`username`:             `anyone`,
		`password`:             `anything`,
		connFactLoginThrottled: `y`,
	}))
}

// The throttle fact follows the connection's source: MaxFailures failures
// from one IP set it, and another IP is unaffected.
func TestConnFactLoginThrottled_FollowsTheSource(t *testing.T) {
	const a, b connections.ConnectionId = 97701, 97702
	connections.RegisterTestWebsocketConnection(a)
	connections.RegisterTestWebsocketConnection(b)
	connections.Get(a).SetClientIP(`198.51.100.10`)
	connections.Get(b).SetClientIP(`198.51.100.11`)
	t.Cleanup(func() {
		connections.UnregisterTestConnection(a)
		connections.UnregisterTestConnection(b)
		gameLoginThrottle.ResetForTest()
	})
	gameLoginThrottle.ResetForTest()

	for range auththrottle.MaxFailures {
		noteLoginFailure(a)
	}
	ra, rb := map[string]string{}, map[string]string{}
	writeConnFacts(ra, a)
	writeConnFacts(rb, b)
	require.Equal(t, `y`, ra[connFactLoginThrottled])
	require.Equal(t, `n`, rb[connFactLoginThrottled])

	clearLoginFailures(a)
	require.False(t, loginThrottled(a))
}
