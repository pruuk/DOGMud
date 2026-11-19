package users

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/pager"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/ansitags"
	"github.com/stretchr/testify/require"
)

var (
	eraseFirstRow = term.AnsiMoveCursorColumn.String() + term.AnsiEraseLine.String()
	eraseRowAbove = term.AnsiMoveCursorUp.String() + term.AnsiEraseLine.String()
)

// promptEraseUser is a telnet reader whose client reported cols columns
// (NAWS); cols 0 means it reported nothing.
func promptEraseUser(t *testing.T, conn uint64, cols uint32) *UserRecord {
	t.Helper()
	connections.RegisterTestConnection(conn)
	t.Cleanup(func() { connections.UnregisterTestConnection(conn) })
	if cols > 0 {
		cs := connections.GetClientSettings(conn)
		cs.Display.ScreenWidth = cols
		cs.NAWSReported = true
		connections.OverwriteClientSettings(conn, cs)
	}
	return NewTestUser(9720, "eraser", "Eraser", conn)
}

// #922: a prompt that wraps fills several rows; the next send must clear
// every one of them, or the first rows stay above the new line.
func TestPromptErase_ClearsEveryRowOfAWrappedPrompt(t *testing.T) {
	u := promptEraseUser(t, 97201, 36)
	u.SetUnsentText(strings.Repeat("x", 60), ``)

	first := u.GetCommandPrompt()
	require.True(t, strings.HasPrefix(first, eraseFirstRow), "a prompt starts by clearing its row")
	require.False(t, strings.HasPrefix(first, eraseFirstRow+eraseRowAbove), "nothing was on screen above it yet")

	body := WithoutPromptErase(first)
	rows := pager.Rows(ansitags.Parse(body, ansitags.StripTags), 36)
	require.GreaterOrEqual(t, rows, 2, "the fixture prompt must wrap at 36 columns")
	require.Equal(t, rows, u.PromptRows())

	want := eraseFirstRow + strings.Repeat(eraseRowAbove, rows-1)
	require.Equal(t, want, u.PromptErase(), "the erase climbs every row the prompt filled")
	require.Equal(t, eraseFirstRow, u.PromptErase(), "once erased, the next send clears one row")

	// A redraw after the prompt was erased clears one row, then counts its own.
	again := u.GetCommandPrompt()
	require.True(t, strings.HasPrefix(again, eraseFirstRow))
	require.False(t, strings.HasPrefix(again, eraseFirstRow+eraseRowAbove))
	require.Equal(t, rows, u.PromptRows())

	// A redraw over a prompt still on screen clears all of its rows.
	over := u.GetCommandPrompt()
	require.True(t, strings.HasPrefix(over, want))
	require.Equal(t, body, WithoutPromptErase(over), "the same prompt compares equal without its erase")
}

// With no width reported the rows are unknown, so only one row is cleared:
// guessing could erase a line of game text above a prompt that did not wrap.
func TestPromptErase_UnknownWidthClearsOneRow(t *testing.T) {
	u := promptEraseUser(t, 97202, 0)
	u.SetUnsentText(strings.Repeat("x", 200), ``)

	u.GetCommandPrompt()
	require.Equal(t, 1, u.PromptRows())
	require.Equal(t, eraseFirstRow, u.PromptErase())
}

func TestRestorePromptRows(t *testing.T) {
	u := promptEraseUser(t, 97203, 36)
	u.RestorePromptRows(3)
	require.Equal(t, 3, u.PromptRows())
	require.Equal(t, eraseFirstRow+eraseRowAbove+eraseRowAbove, u.PromptErase())
}
