package inputhandlers

import (
	"errors"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// No step may use a reserved connection-fact key as its ID: the handler
// overwrites those keys before every condition.
func TestLoginSteps_NoStepUsesAConnFactID(t *testing.T) {
	for _, step := range loginSteps() {
		require.False(t, strings.HasPrefix(step.ID, connFactPrefix), step.ID)
	}
}

func screenReaderStep(t *testing.T) *PromptStep {
	t.Helper()
	for _, step := range loginSteps() {
		if step.ID == `screen-reader-new` {
			return step
		}
	}
	t.Fatal("no screen-reader-new step")
	return nil
}

// Asked only of a new telnet player whose client sent no MTTS.
func TestScreenReaderQuestion_AskedOnlyWithoutAnAnswer(t *testing.T) {
	cond := screenReaderStep(t).Condition
	base := func(ws, mtts string) map[string]string {
		return map[string]string{`username`: `new`, connFactWebsocket: ws, connFactMTTS: mtts}
	}
	require.True(t, cond(base(`n`, `n`)), "telnet without MTTS is asked")
	require.False(t, cond(base(`y`, `n`)), "web is never asked")
	require.False(t, cond(base(`n`, `y`)), "any MTTS reply answers it")
	login := base(`n`, `n`)
	login[`username`] = `bob`
	require.False(t, cond(login), "a login is never asked")
}

// The answer when asked, else what the connection reported.
func TestNewUserScreenReader(t *testing.T) {
	require.True(t, newUserScreenReader(map[string]string{`screen-reader-new`: `y`}, false))
	require.False(t, newUserScreenReader(map[string]string{`screen-reader-new`: `n`}, false))
	require.True(t, newUserScreenReader(map[string]string{}, true))
	require.False(t, newUserScreenReader(map[string]string{}, false))
}

// The connection's facts are written as y or n under the reserved keys.
func TestWriteConnFacts(t *testing.T) {
	const web, tel connections.ConnectionId = 97401, 97402
	connections.RegisterTestWebsocketConnection(web)
	connections.RegisterTestConnection(tel)
	t.Cleanup(func() {
		connections.UnregisterTestConnection(web)
		connections.UnregisterTestConnection(tel)
	})
	// A real websocket always has a proxy-resolved client IP; the test one
	// has no socket to read it from.
	connections.Get(web).SetClientIP(`192.0.2.1`)
	cs := connections.GetClientSettings(web)
	cs.ScreenReader = true
	connections.OverwriteClientSettings(web, cs)
	cs = connections.GetClientSettings(tel)
	cs.MTTSReported = true
	connections.OverwriteClientSettings(tel, cs)

	r := map[string]string{}
	writeConnFacts(r, web)
	require.Equal(t, map[string]string{connFactWebsocket: `y`, connFactMTTS: `n`, connFactLoginThrottled: `n`}, r)
	r = map[string]string{}
	writeConnFacts(r, tel)
	require.Equal(t, map[string]string{connFactWebsocket: `n`, connFactMTTS: `y`, connFactLoginThrottled: `n`}, r)
}

// The handler really writes the facts before a condition: a telnet client
// that answered MTTS skips the question. Leaving the writeConnFacts call
// out of advanceAndSendPrompt fails this. (A telnet test connection: a
// skipped step sends nothing, and the sequence completes.)
func TestAdvanceAndSendPrompt_SkipsTheQuestionOnMTTS(t *testing.T) {
	const tel connections.ConnectionId = 97403
	connections.RegisterTestConnection(tel)
	t.Cleanup(func() { connections.UnregisterTestConnection(tel) })
	cs := connections.GetClientSettings(tel)
	cs.MTTSReported = true
	connections.OverwriteClientSettings(tel, cs)

	state := &PromptHandlerState{
		Steps:            []*PromptStep{screenReaderStep(t)},
		CurrentStepIndex: -1,
		Results:          map[string]string{`username`: `new`},
	}
	require.True(t, advanceAndSendPrompt(state, &connections.ClientInput{ConnectionId: tel}), "the only step is skipped")
	require.Equal(t, `y`, state.Results[connFactMTTS])
}

func drainSettingEvents(t *testing.T) {
	t.Helper()
	events.DrainQueuedByTypeForTest(`UserSettingChanged`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`UserSettingChanged`) })
}

// A connection's report turns an account's setting on, never off, and says so.
func TestLoginWithScreenReader_OnNeverOff(t *testing.T) {
	drainSettingEvents(t)
	same := func(u *users.UserRecord) (*users.UserRecord, string, error) { return u, ``, nil }

	off := &users.UserRecord{UserId: 8}
	_, _, lines, err := loginWithScreenReader(off, true, same)
	require.NoError(t, err)
	require.True(t, off.ScreenReader)
	require.Equal(t, `light`, off.CombatVerbosity)
	require.Equal(t, append(append([]string{}, users.ScreenReaderOnNotices...), users.ScreenReaderCombatLines...), lines)

	on := &users.UserRecord{UserId: 9, ScreenReader: true}
	_, _, lines, err = loginWithScreenReader(on, true, same)
	require.NoError(t, err)
	require.Empty(t, lines, "already on: nothing to say")
	_, _, lines, err = loginWithScreenReader(on, false, same)
	require.NoError(t, err)
	require.Empty(t, lines)
	require.True(t, on.ScreenReader, "a connection never turns the setting off")

	quiet := &users.UserRecord{UserId: 10}
	_, _, lines, err = loginWithScreenReader(quiet, false, same)
	require.NoError(t, err)
	require.Empty(t, lines)
	require.False(t, quiet.ScreenReader)
}

// A failed login returns the error, sends no lines and queues no report.
func TestLoginWithScreenReader_FailedLogin(t *testing.T) {
	drainSettingEvents(t)
	events.DrainQueuedByTypeForTest(`ScreenReaderReported`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`ScreenReaderReported`) })

	_, _, lines, err := loginWithScreenReader(&users.UserRecord{UserId: 14}, true,
		func(*users.UserRecord) (*users.UserRecord, string, error) { return nil, ``, errors.New("no") })
	require.EqualError(t, err, "no")
	require.Nil(t, lines)
	require.Empty(t, events.DrainQueuedByTypeForTest(`ScreenReaderReported`))
}

// A reconnect to a link-dead session continues the user already in the world
// (users.go:240-242), not the record loaded for this login. That user is read
// by MainWorker every round, so this goroutine must not write it: the report
// goes to MainWorker as ScreenReaderReported, and no lines are sent here.
func TestLoginWithScreenReader_LinkDeadReconnect(t *testing.T) {
	drainSettingEvents(t)
	events.DrainQueuedByTypeForTest(`ScreenReaderReported`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`ScreenReaderReported`) })
	loaded := &users.UserRecord{UserId: 11}
	inMemory := &users.UserRecord{UserId: 11}
	fakeLogin := func(u *users.UserRecord) (*users.UserRecord, string, error) {
		require.Same(t, loaded, u)
		require.True(t, u.ScreenReader, "the report is applied before the user is published")
		return inMemory, ``, nil
	}
	got, _, lines, err := loginWithScreenReader(loaded, true, fakeLogin)
	require.NoError(t, err)
	require.Same(t, inMemory, got)
	require.False(t, inMemory.ScreenReader, "the live user is not written from this goroutine")
	require.Empty(t, lines, "the hook tells the player, so login sends nothing")
	queued := events.DrainQueuedByTypeForTest(`ScreenReaderReported`)
	require.Equal(t, []events.Event{events.ScreenReaderReported{UserId: 11}}, queued)

	// No report: nothing queued.
	_, _, _, err = loginWithScreenReader(&users.UserRecord{UserId: 11}, false, fakeLoginNoCheck(inMemory))
	require.NoError(t, err)
	require.Empty(t, events.DrainQueuedByTypeForTest(`ScreenReaderReported`))
}

func fakeLoginNoCheck(ret *users.UserRecord) func(*users.UserRecord) (*users.UserRecord, string, error) {
	return func(*users.UserRecord) (*users.UserRecord, string, error) { return ret, ``, nil }
}

// The ordinary login: the same record comes back, turned on once.
func TestLoginWithScreenReader_Ordinary(t *testing.T) {
	drainSettingEvents(t)
	loaded := &users.UserRecord{UserId: 12}
	got, _, lines, err := loginWithScreenReader(loaded, true, func(u *users.UserRecord) (*users.UserRecord, string, error) { return u, ``, nil })
	require.NoError(t, err)
	require.Same(t, loaded, got)
	require.True(t, got.ScreenReader)
	require.Equal(t, append(append([]string{}, users.ScreenReaderOnNotices...), users.ScreenReaderCombatLines...), lines)

	quiet := &users.UserRecord{UserId: 13}
	_, _, lines, err = loginWithScreenReader(quiet, false, func(u *users.UserRecord) (*users.UserRecord, string, error) { return u, ``, nil })
	require.NoError(t, err)
	require.Empty(t, lines)
	require.False(t, quiet.ScreenReader)
}

// Creation: asked and answered yes gets only the combat lines (the player just
// said so); not asked but reported also hears that the mode is on and how to
// turn it off; neither gets nothing.
func TestCreationScreenReaderLines(t *testing.T) {
	drainSettingEvents(t)

	asked := &users.UserRecord{}
	lines := creationScreenReaderLines(asked, map[string]string{`screen-reader-new`: `y`}, false)
	require.True(t, asked.ScreenReader)
	require.Equal(t, users.ScreenReaderCombatLines, lines)

	reported := &users.UserRecord{}
	lines = creationScreenReaderLines(reported, map[string]string{}, true)
	require.True(t, reported.ScreenReader)
	require.Equal(t, append(append([]string{}, users.ScreenReaderOnNotices...), users.ScreenReaderCombatLines...), lines)

	no := &users.UserRecord{}
	require.Empty(t, creationScreenReaderLines(no, map[string]string{`screen-reader-new`: `n`}, true))
	require.False(t, no.ScreenReader, "an answered no wins over a report")
}
