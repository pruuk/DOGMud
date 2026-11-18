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

// The kick question checks no password: a password check there ran before
// the login's attempt count and told a locked-out or banned source when a
// guess was right. An account nobody is playing is never asked about.
func TestKickQuestion_ChecksNoPassword(t *testing.T) {
	step := kickStep(t)
	require.False(t, step.Condition(map[string]string{`username`: `nobody-online-here`, `password`: `anything`}))
	require.False(t, step.Condition(map[string]string{`username`: `new`}))
}

// Attempts are counted per source before any password work: MaxFailures
// attempts lock the source out, another source is unaffected.
func TestLoginAttempt_LocksTheSourceOut(t *testing.T) {
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

	for i := range auththrottle.MaxFailures {
		require.True(t, loginAttempt(a), "attempt %d refused", i+1)
	}
	require.False(t, loginAttempt(a), "attempt past the limit allowed")
	require.True(t, loginAttempt(b), "another source was locked out")
}

// A good login gives back only its own attempt. Logging into an account of
// one's own between guesses must not reset the count: anyone can make an
// account, so a reset on success would make the throttle useless.
func TestLoginSucceeded_DoesNotResetOtherFailures(t *testing.T) {
	const a connections.ConnectionId = 97703
	connections.RegisterTestWebsocketConnection(a)
	connections.Get(a).SetClientIP(`198.51.100.12`)
	t.Cleanup(func() {
		connections.UnregisterTestConnection(a)
		gameLoginThrottle.ResetForTest()
	})
	gameLoginThrottle.ResetForTest()

	for range auththrottle.MaxFailures - 1 {
		require.True(t, loginAttempt(a)) // wrong guesses
	}
	require.True(t, loginAttempt(a)) // a login to the guesser's own account...
	loginSucceeded(a)                // ...gives back only itself
	require.True(t, loginAttempt(a)) // the next guess is the 5th failure
	require.False(t, loginAttempt(a), "a success between guesses reset the count")
}
