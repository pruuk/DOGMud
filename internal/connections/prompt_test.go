package connections

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/stretchr/testify/require"
)

func TestEraseFor(t *testing.T) {
	up := eraseRowAbove
	for _, tt := range []struct {
		name     string
		rec      promptRecord
		curWidth int
		want     string
	}{
		{"no prompt on screen", promptRecord{}, 36, eraseOneRow},
		{"a prompt that fits", promptRecord{on: true, cols: 30, width: 36}, 36, eraseOneRow},
		{"exactly the width is one row", promptRecord{on: true, cols: 36, width: 36}, 36, eraseOneRow},
		{"two rows", promptRecord{on: true, cols: 60, width: 36}, 36, eraseOneRow + up},
		{"three rows", promptRecord{on: true, cols: 80, width: 36}, 36, eraseOneRow + up + up},
		{"widened since: a reflowed prompt is one row", promptRecord{on: true, cols: 60, width: 36}, 100, eraseOneRow},
		{"narrowed since: the rows as drawn", promptRecord{on: true, cols: 60, width: 36}, 20, eraseOneRow + up},
		{"width unknown now", promptRecord{on: true, cols: 60, width: 36}, 0, eraseOneRow},
		{"width unknown when drawn", promptRecord{on: true, cols: 60, width: 0}, 36, eraseOneRow},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, eraseFor(tt.rec, tt.curWidth))
		})
	}
}

func TestOutOfBand(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    string
		ws   bool
		want bool
	}{
		{"telnet command", string([]byte{term.TELNET_IAC, 250, 201}), false, true},
		{"web GMCP", `!!GMCP(Char.Vitals {"hp":1})`, true, true},
		{"web sound", `!!SOUND(hit.mp3 T=combat V=50)`, true, true},
		{"web client command", `TEXTMASK:on`, true, true},
		{"game text", "A cold wind blows.\r\n", true, false},
		{"a colon at nine is not a command", `Sable sa: hello there`, true, false},
		{"lower case is not a command", `textmask:on`, true, false},
		{"an enter echo", "\r\n", false, false},
		// A telnet client shows these as text (second review of #924).
		{"!! on telnet is text", `!!Wow, a dragon!!`, false, false},
		{"a command form on telnet is text", `WARNING!: the bridge is out`, false, false},
		{"capitals and a colon on telnet are text", `ATTENTION: the gates close`, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, outOfBand([]byte(tt.p), tt.ws))
		})
	}
}

// MinCols counts every visible character as one column, so a prompt is
// never measured wider than any terminal draws it (second review of #924).
func TestMinCols(t *testing.T) {
	for _, tt := range []struct {
		name string
		text string
		want int
	}{
		{"plain", "HP:100", 6},
		{"escapes take nothing", "\x1b[1;31mHP\x1b[0m:100", 6},
		{"an emoji is one", "\U0001F6E1 HP", 4},
		{"a wide letter is one", "日本", 2},
		{"bars are one each", "██░░", 4},
		{"a combining mark is none", "é", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MinCols(tt.text))
		})
	}
}

// promptScreen is a telnet connection over a pipe that reported width cols by
// NAWS (0: reported nothing), and what its client receives.
func promptScreen(t *testing.T, cols uint32) (ConnectionId, *pipeReader) {
	t.Helper()
	id, client := addPipeConnection(t)
	if cols > 0 {
		UpdateClientSettings(id, func(cs *ClientSettings) {
			cs.Display.ScreenWidth = cols
			cs.NAWSReported = true
		})
	}
	return id, startPipeReader(client)
}

// rowsClearedBefore is how many rows the erase written just before marker
// clears: 1 for column 1 and erase line, plus one per up-and-erase.
func rowsClearedBefore(t *testing.T, r *pipeReader, marker string) int {
	t.Helper()
	var text string
	require.Eventually(t, func() bool {
		text = r.text()
		return strings.Contains(text, marker)
	}, 2*time.Second, 5*time.Millisecond, "never received %q", marker)
	before := text[:strings.Index(text, marker)]
	rows := 1
	for strings.HasSuffix(before, eraseRowAbove) {
		before = strings.TrimSuffix(before, eraseRowAbove)
		rows++
	}
	require.True(t, strings.HasSuffix(before, eraseOneRow), "no erase before %q", marker)
	return rows
}

var wrapped = strings.Repeat("p", 60) // 2 rows at 36 columns

// #922: text that replaces a prompt which wrapped clears all of its rows.
func TestPrompt_AWrappedPromptIsClearedWhole(t *testing.T) {
	id, r := promptScreen(t, 36)
	SendPrompt(id, []byte(wrapped), 60)
	SendReplacingPrompt(id, []byte("<M1>A cold wind blows.\n"))
	require.Equal(t, 2, rowsClearedBefore(t, r, "<M1>"))

	// It was replaced: the next line clears one row.
	SendReplacingPrompt(id, []byte("<M2>Again.\n"))
	require.Equal(t, 1, rowsClearedBefore(t, r, "<M2>"))
}

// A redraw clears the prompt it replaces, then is recorded itself.
func TestPrompt_ARedrawClearsTheOldPrompt(t *testing.T) {
	id, r := promptScreen(t, 36)
	SendPrompt(id, []byte(wrapped), 60)
	SendPrompt(id, []byte("<M1>"+strings.Repeat("q", 80)), 84)
	require.Equal(t, 2, rowsClearedBefore(t, r, "<M1>"))
	SendReplacingPrompt(id, []byte("<M2>line\n"))
	require.Equal(t, 3, rowsClearedBefore(t, r, "<M2>"))
}

// Review finding 1: pressing Enter echoes a new line, leaving the prompt and
// the typed command above the cursor. The reply must not climb into them.
func TestPrompt_EnterEndsThePrompt(t *testing.T) {
	id, r := promptScreen(t, 36)
	SendPrompt(id, []byte(wrapped), 60)
	SendEcho(id, []byte("kill rat"))
	SendEcho(id, term.CRLF)
	SendReplacingPrompt(id, []byte("<M1>You attack the rat.\n"))
	require.Equal(t, 1, rowsClearedBefore(t, r, "<M1>"))
}

// Characters typed at the prompt extend it: a prompt that fit one row and
// wraps once the player types is cleared whole.
func TestPrompt_TypingExtendsThePrompt(t *testing.T) {
	id, r := promptScreen(t, 36)
	SendPrompt(id, []byte(strings.Repeat("p", 30)), 30)
	SendEcho(id, []byte("look north"))
	SendReplacingPrompt(id, []byte("<M1>A line.\n"))
	require.Equal(t, 2, rowsClearedBefore(t, r, "<M1>"))
}

// A backspace or any control byte ends the record: one row, never too many.
func TestPrompt_ABackspaceEndsTheRecord(t *testing.T) {
	id, r := promptScreen(t, 36)
	SendPrompt(id, []byte(wrapped), 60)
	SendEcho(id, []byte("\b \b"))
	SendReplacingPrompt(id, []byte("<M1>A line.\n"))
	require.Equal(t, 1, rowsClearedBefore(t, r, "<M1>"))
}

// Review finding 2: a write that skips the erase (a reload result, a log
// line) lands after the prompt; the next erase must not climb into it.
func TestPrompt_AnyOtherWriteEndsTheRecord(t *testing.T) {
	id, r := promptScreen(t, 36)
	SendPrompt(id, []byte(wrapped), 60)
	SendTo([]byte("Reloading flat files... done.\n"), id)
	SendReplacingPrompt(id, []byte("<M1>A line.\n"))
	require.Equal(t, 1, rowsClearedBefore(t, r, "<M1>"))
}

// Telnet negotiation and GMCP show nothing, so the prompt is still there.
func TestPrompt_OutOfBandWritesKeepTheRecord(t *testing.T) {
	id, r := promptScreen(t, 36)
	SendPrompt(id, []byte(wrapped), 60)
	SendTo([]byte{term.TELNET_IAC, 250, 201, 'x', term.TELNET_IAC, 240}, id)
	SendReplacingPrompt(id, []byte("<M1>A line.\n"))
	require.Equal(t, 2, rowsClearedBefore(t, r, "<M1>"))
}

// Review finding 3: a screen that widened since may have reflowed the prompt
// to one row; clearing two would erase a line of game text.
func TestPrompt_AWidenedScreenClearsTheReflowedRows(t *testing.T) {
	id, r := promptScreen(t, 36)
	SendPrompt(id, []byte(wrapped), 60)
	UpdateClientSettings(id, func(cs *ClientSettings) { cs.Display.ScreenWidth = 100 })
	SendReplacingPrompt(id, []byte("<M1>A line.\n"))
	require.Equal(t, 1, rowsClearedBefore(t, r, "<M1>"))
}

// A prompt dropped while output is frozen (copyover) never reached the
// screen, so it is not recorded; after a failed copyover thaws output, the
// next line must not climb rows that were never drawn (second review).
func TestPrompt_ADroppedPromptIsNotRecorded(t *testing.T) {
	t.Cleanup(ThawOutputAfterFailedCopyover)
	id, r := promptScreen(t, 36)
	freezeOutput()
	SendPrompt(id, []byte(wrapped), 60)
	ThawOutputAfterFailedCopyover()
	SendReplacingPrompt(id, []byte("<M1>A line.\n"))
	require.Equal(t, 1, rowsClearedBefore(t, r, "<M1>"), "a dropped prompt")

	// A dropped echo does not extend the prompt: 30 columns drawn, 10 typed
	// but never shown, so one row is cleared, not two.
	SendPrompt(id, []byte(strings.Repeat("p", 30)), 30)
	freezeOutput()
	SendEcho(id, []byte("look north"))
	ThawOutputAfterFailedCopyover()
	SendReplacingPrompt(id, []byte("<M2>A line.\n"))
	require.Equal(t, 1, rowsClearedBefore(t, r, "<M2>"), "a dropped echo")
}

// ASCII mode swaps or drops characters after the prompt was measured, so a
// prompt there is not recorded (second review).
func TestPrompt_AsciiModeIsNotRecorded(t *testing.T) {
	id, r := promptScreen(t, 36)
	UpdateClientSettings(id, func(cs *ClientSettings) { cs.AsciiMode = true })
	SendPrompt(id, []byte(wrapped), 60)
	SendReplacingPrompt(id, []byte("<M1>A line.\n"))
	require.Equal(t, 1, rowsClearedBefore(t, r, "<M1>"))
}

// With no width reported the rows are unknown: one row, as always.
func TestPrompt_UnknownWidthClearsOneRow(t *testing.T) {
	id, r := promptScreen(t, 0)
	SendPrompt(id, []byte(strings.Repeat("p", 200)), 200)
	SendReplacingPrompt(id, []byte("<M1>A line.\n"))
	require.Equal(t, 1, rowsClearedBefore(t, r, "<M1>"))
}
