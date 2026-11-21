package connections

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/mattn/go-runewidth"
)

// The prompt record (#922). A prompt that wraps fills several screen rows,
// and the text that replaces it must clear all of them or the first rows stay
// on screen above the new line. Only the connection knows what it last wrote,
// so it keeps the record, set and read under the same lock as the write:
//
//   - SendPrompt replaces the prompt on screen and records the new one: the
//     lines before its last (a prompt the game wrapped), its last line's
//     visible columns, and the width the client reported when it was drawn.
//   - SendEcho writes the characters a player types at the prompt, which
//     extend its last line; anything but printable characters ends the
//     record.
//   - SendReplacingPrompt writes text over the prompt and ends the record.
//   - Every other write that shows on screen ends the record. Out-of-band
//     writes (telnet negotiation, GMCP, sound, web client commands) do not.
//
// The erase clears every row the recorded prompt fills; with no record, or
// at an unknown width, it clears one row, as the game always did. Every doubt
// falls back to one row: a stray row left behind is harmless, while climbing
// too far would erase game text.
type promptRecord struct {
	on    bool
	above int // lines of the prompt before its last line
	cols  int // visible columns of the prompt's last line and what was typed after it
	width int // the reported screen width when the prompt was drawn
}

// eraseOneRow returns to column 1 and clears the row.
var eraseOneRow = term.AnsiMoveCursorColumn.String() + term.AnsiEraseLine.String()

// eraseRowAbove moves up a row and clears it.
var eraseRowAbove = term.AnsiMoveCursorUp.String() + term.AnsiEraseLine.String()

// escapeSeqRe matches one ANSI control sequence, which takes no column.
var escapeSeqRe = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

// MinCols is the fewest columns text can take on one unbroken row: escape
// sequences take none, and every other character that shows takes one, even
// one a terminal may draw two wide (an emoji, an ambiguous-width bar).
// Terminals disagree on those widths, so the prompt record measures with
// this lower bound: counting too few rows leaves a stray row, counting too
// many would erase game text (#922 second review).
func MinCols(text string) int {
	cols := 0
	for _, r := range escapeSeqRe.ReplaceAllString(text, ``) {
		if runewidth.RuneWidth(r) > 0 {
			cols++
		}
	}
	return cols
}

// promptRows is the rows cols visible columns fill on a screen width columns
// wide: at least 1.
func promptRows(cols, width int) int {
	if width < 1 {
		return 1
	}
	return max(1, (cols+width-1)/width)
}

// eraseFor is the sequence that clears the prompt rec records, on a screen
// now reporting curWidth (0 when unknown). A screen whose width changed since
// the prompt was drawn may or may not have reflowed it, so the smaller row
// count is used: it is never more than the rows actually on screen.
//
// A prompt the game wrapped onto several lines clears each line above its
// last as one row (a line break is a row on any width, and a line the game
// wrapped fits it, so one row is never more than is there), then the rows of
// its last line by the rule above.
func eraseFor(rec promptRecord, curWidth int) string {
	if !rec.on || rec.width < 1 || curWidth < 1 {
		return eraseOneRow
	}
	rows := rec.above + min(promptRows(rec.cols, rec.width), promptRows(rec.cols, curWidth))
	return eraseOneRow + strings.Repeat(eraseRowAbove, rows-1)
}

// ReportedScreenWidth is the width in columns a connection's client reported
// and whether it reported one: a websocket's columns, a telnet client's NAWS
// width, or the width a cursor position report set (Display.ScreenWidth
// without NAWSReported). A connection starts at DefaultScreenWidth, so a
// telnet width equal to it without NAWS is taken as not reported. The pager,
// the wrap width (internal/users) and the prompt record all use this rule.
func ReportedScreenWidth(id ConnectionId) (int, bool) {
	lock.RLock()
	cd, ok := netConnections[id]
	if !ok {
		lock.RUnlock()
		return 0, false
	}
	ws := cd.IsWebSocket()
	cs := cd.clientSettings
	lock.RUnlock()

	if ws || cs.NAWSReported ||
		(cs.Display.ScreenWidth != 0 && cs.Display.ScreenWidth != DefaultScreenWidth) {
		return cs.Display.GetScreenWidth(), true
	}
	return 0, false
}

// SendPrompt writes a prompt over the one on screen: the erase for the
// recorded prompt, then b. b may be several lines (a prompt the game wrapped);
// the connection counts its line breaks itself. cols is the visible width in
// columns of b's last line, measured by the caller with MinCols (markup
// removed first). The new prompt is recorded only when the client reported a
// width, the bytes were written (a frozen queue during copyover writes
// nothing), and the connection is not in ASCII mode, whose substitutions
// change the width after it was measured.
func SendPrompt(id ConnectionId, b []byte, cols int) {
	width, _ := ReportedScreenWidth(id)
	ascii := GetClientSettings(id).AsciiMode // read before promptMu: no lock is taken under it
	above := bytes.Count(b, []byte{'\n'})
	cd := lookup(id)
	if cd == nil {
		return
	}
	cd.promptMu.Lock()
	n, err := cd.writeNLocked(append([]byte(eraseFor(cd.prompt, width)), b...))
	cd.prompt = promptRecord{on: width > 0 && n > 0 && !ascii, above: above, cols: cols, width: width}
	cd.promptMu.Unlock()
	afterWrite(id, cd, err, "SendPrompt()")
}

// SendReplacingPrompt writes b over the prompt on screen: the erase for the
// recorded prompt, then b. The prompt is gone afterwards.
func SendReplacingPrompt(id ConnectionId, b []byte) {
	width, _ := ReportedScreenWidth(id)
	cd := lookup(id)
	if cd == nil {
		return
	}
	cd.promptMu.Lock()
	err := cd.writeLocked(append([]byte(eraseFor(cd.prompt, width)), b...))
	cd.prompt = promptRecord{}
	cd.promptMu.Unlock()
	afterWrite(id, cd, err, "SendReplacingPrompt()")
}

// SendEcho writes characters a player typed back to them. Printable text
// extends the recorded prompt, since it is drawn on the prompt's line; any
// other byte (Enter, a backspace, a control sequence) ends the record.
func SendEcho(id ConnectionId, b []byte) {
	cd := lookup(id)
	if cd == nil {
		return
	}
	cd.promptMu.Lock()
	n, err := cd.writeNLocked(b)
	if cd.prompt.on && n > 0 && printable(b) {
		cd.prompt.cols += MinCols(string(b))
	} else {
		cd.prompt = promptRecord{}
	}
	cd.promptMu.Unlock()
	afterWrite(id, cd, err, "SendEcho()")
}

// printable reports whether b is valid UTF-8 holding only printable
// characters.
func printable(b []byte) bool {
	if !utf8.Valid(b) {
		return false
	}
	for _, r := range string(b) {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// outOfBand reports whether p is a write that shows nothing on screen: a
// telnet command (IAC), or on a websocket a web client payload (!!GMCP(,
// !!SOUND(, !!MUSIC() or command (eight capital letters and a colon, as
// TEXTMASK:), matched as the web client itself matches them. A telnet
// client shows those two forms as text, so for telnet only IAC counts: a
// mistake here may only ever end the record, never keep it.
func outOfBand(p []byte, websocket bool) bool {
	if len(p) == 0 {
		return true
	}
	if p[0] == term.TELNET_IAC {
		return true
	}
	if !websocket {
		return false
	}
	if len(p) > 9 && p[0] == '!' && p[1] == '!' {
		return true
	}
	if len(p) > 9 && p[8] == ':' {
		for _, c := range p[:8] {
			if c < 'A' || c > 'Z' {
				return false
			}
		}
		return true
	}
	return false
}

// lookup is the connection for id, or nil.
func lookup(id ConnectionId) *ConnectionDetails {
	lock.RLock()
	defer lock.RUnlock()
	return netConnections[id]
}

// afterWrite reports a failed write and removes the connection, as SendTo
// does.
func afterWrite(id ConnectionId, cd *ConnectionDetails, err error, where string) {
	if err == nil {
		return
	}
	reportSendError(where, writeTarget{id: id, cd: cd}, err)
	// Already gone is the outcome wanted, as in SendTo.
	_ = Remove(id)
}
