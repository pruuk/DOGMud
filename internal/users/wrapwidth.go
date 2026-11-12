package users

import (
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/connections"
)

const (
	// MinWrapWidth is the narrowest a web session wraps. WCAG 2.2 AA reflow at
	// 320px needs 30 columns.
	MinWrapWidth = 30
	// MaxAutoWrapWidth is the widest a web session wraps: the game window
	// shows at most 80 columns.
	MaxAutoWrapWidth = 80
)

// ParseReportedCols reads a column count a web client sent as text: the
// cols value on its websocket address (/ws?cols=57), sent so the connect
// splash is drawn before its Client.Display report arrives. Only a whole
// number counts; a fraction, a unit, a space or an empty value is refused.
// The count is not clamped here; StoreReportedCols clamps it.
func ParseReportedCols(s string) (int, bool) {
	cols, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return cols, true
}

// StoreReportedCols stores a web client's reported column count as its
// connection's screen width, clamped to MinWrapWidth..MaxAutoWrapWidth, and
// returns the width. It writes only when the width differs. Both places a
// web client reports its columns use it: the websocket address on connect
// (main.go) and Client.Display (modules/gmcp). The caller checks that the
// connection is a websocket.
func StoreReportedCols(id connections.ConnectionId, cols int) int {
	width := min(max(cols, MinWrapWidth), MaxAutoWrapWidth)
	cs := connections.GetClientSettings(id)
	if cs.Display.GetScreenWidth() != width {
		cs.Display.ScreenWidth = uint32(width)
		connections.OverwriteClientSettings(id, cs)
	}
	return width
}

// NarrowFormWidth is the width of the narrow form of a screen built in Go
// for a reader width columns wide (web client base spec, section 2): 42, or
// 30 below that (or the reader's width, if narrower still). It is a band
// and measures nothing, so a 40 or 41 column reader gets 30 where
// templates.Process, which measures each form, may pick the 42 form.
func NarrowFormWidth(width int) int {
	if width >= 42 {
		return 42
	}
	return min(width, 30)
}

// RedrawnFormWidth is the width to draw a Go-built screen to whose
// 80-column layout was itself redrawn to fit 80 (plan 3b-3: the craft
// screens, `guild list`, scan, macros): that layout at 80 and wider,
// whatever the reader's width past 80, else NarrowFormWidth.
func RedrawnFormWidth(width int) int {
	if width >= MaxAutoWrapWidth {
		return MaxAutoWrapWidth
	}
	return NarrowFormWidth(width)
}

// WrapWidth is the column width text for this user wraps at, and the only
// source of one: every send and every layout that wraps reads it.
//
// A web session wraps at the smallest of the columns its client reported, 80,
// and the saved LineWidth when one is set, and never below MinWrapWidth, so
// `set linewidth` can only narrow it. The reported value is clamped again
// here, not only where it arrives, because nothing else stops a websocket
// client sending a raw telnet screen-size report. Every other session, and a
// nil receiver, gets GetLineWidth.
func (u *UserRecord) WrapWidth() int {
	if u == nil || !connections.IsWebsocket(u.connectionId) {
		return u.GetLineWidth()
	}
	width := min(u.ClientSettings().Display.GetScreenWidth(), MaxAutoWrapWidth)
	if u.LineWidth > 0 {
		width = min(width, u.LineWidth)
	}
	return max(width, MinWrapWidth)
}
