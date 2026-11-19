package hooks

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #922: on a narrow telnet screen a prompt that wraps (here a long line the
// player is typing; in the playtest the pager's stop line) fills several
// rows. A line that arrives must clear all of them before it prints, or the
// prompt's first rows stay on screen above it. An unchanged redraw that is
// skipped leaves the screen, and so the count, as it was.
func TestRedraw_AWrappedPromptIsClearedWhole(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	events.DrainAllQueuedEventsForTest()

	connId, screen := captureConnection(t)
	connections.UpdateClientSettings(connId, func(cs *connections.ClientSettings) {
		cs.Display.ScreenWidth = 36
		cs.NAWSReported = true
	})
	u := users.NewTestUser(1, "alice", "Aliceia", connId)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: u})
	defer cleanupUsers()

	u.SetUnsentText(strings.Repeat("x", 60), ``)
	RedrawPrompt_SendRedraw(events.RedrawPrompt{UserId: 1, OnlyIfChanged: true})
	rows := u.PromptRows()
	require.GreaterOrEqual(t, rows, 2, "the fixture prompt must wrap at 36 columns")

	// An unchanged redraw is skipped and keeps the count of what is on screen.
	RedrawPrompt_SendRedraw(events.RedrawPrompt{UserId: 1, OnlyIfChanged: true})
	require.Equal(t, rows, u.PromptRows(), "a skipped redraw changed the count")

	// Wait for the prompt to arrive, so the next bytes are the message's.
	require.Eventually(t, func() bool { return strings.Contains(screen.bytes(), "xxxx") }, time.Second, 10*time.Millisecond)
	before := len(screen.bytes())

	Message_SendMessage(events.Message{UserId: 1, Text: "A cold wind blows.\n"})

	wantErase := term.AnsiMoveCursorColumn.String() + term.AnsiEraseLine.String() +
		strings.Repeat(term.AnsiMoveCursorUp.String()+term.AnsiEraseLine.String(), rows-1)
	require.Eventually(t, func() bool {
		return strings.Contains(screen.bytes()[before:], "A cold wind blows.")
	}, time.Second, 10*time.Millisecond)
	require.True(t, strings.HasPrefix(screen.bytes()[before:], wantErase+"A cold wind blows."),
		"the line must clear all %d rows of the prompt first; got %q", rows, screen.bytes()[before:])
}

// An open question that wraps is erased by an arriving line, and its redraw
// is then skipped because the question is unchanged (the 2026-07-17 rule
// against re-printing it after every chat line). Nothing was drawn, so the
// next line must clear one row only: clearing the question's rows again
// would wipe the chat line above.
func TestRedraw_ASkippedQuestionRedrawLeavesOneRowToClear(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	events.DrainAllQueuedEventsForTest()

	connId, _ := captureConnection(t)
	connections.UpdateClientSettings(connId, func(cs *connections.ClientSettings) {
		cs.Display.ScreenWidth = 36
		cs.NAWSReported = true
	})
	u := users.NewTestUser(1, "alice", "Aliceia", connId)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: u})
	defer cleanupUsers()

	p, _ := u.StartPrompt(`portalstake`, ``)
	p.Ask(`How much gold will you stake on the Arena fight tonight?`, []string{})
	RedrawPrompt_SendRedraw(events.RedrawPrompt{UserId: 1})
	require.GreaterOrEqual(t, u.PromptRows(), 2, "the fixture question must wrap at 36 columns")

	Message_SendMessage(events.Message{UserId: 1, Text: "Sable nods.\n"})
	RedrawPrompt_SendRedraw(events.RedrawPrompt{UserId: 1}) // skipped: the same question
	require.Equal(t, 1, u.PromptRows(), "the skipped redraw drew nothing, so one row is left to clear")
}
