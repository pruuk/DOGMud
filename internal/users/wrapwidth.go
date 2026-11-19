package users

import (
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/pager"
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
// number counts (a leading sign is accepted, and a negative is clamped
// later); a fraction, a unit, a space or an empty value is refused.
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
	connections.UpdateClientSettings(id, func(cs *connections.ClientSettings) {
		if cs.Display.GetScreenWidth() != width {
			cs.Display.ScreenWidth = uint32(width)
		}
	})
	return width
}

// StoreReportedRows stores a web client's reported row count as the
// connection's screen height, clamped to pager.MinSize..pager.MaxSize, and
// written only when it differs (#842). Telnet heights come from NAWS instead.
func StoreReportedRows(id connections.ConnectionId, rows int) int {
	height := min(max(rows, pager.MinSize), pager.MaxSize)
	connections.UpdateClientSettings(id, func(cs *connections.ClientSettings) {
		if cs.Display.GetScreenHeight() != height {
			cs.Display.ScreenHeight = uint32(height)
		}
	})
	return height
}

// LoginWrapWidth is the width text sent to a connection before it has a user
// is laid out for: the login prompts, their errors and the yes or no
// questions (internal/inputhandlers). A web client's columns are stored on
// connect (from the cols on its websocket address) and again by
// Client.Display, so a websocket gets them, clamped to
// MinWrapWidth..MaxAutoWrapWidth as WrapWidth clamps them, and 80 when it
// reported none. Every other connection gets 80 whatever its terminal
// reports: the width telnet login text was written for, which inputhandlers
// sends unchanged.
func LoginWrapWidth(id connections.ConnectionId) int {
	if !connections.IsWebsocket(id) {
		return MaxAutoWrapWidth
	}
	width := connections.GetClientSettings(id).Display.GetScreenWidth()
	return min(max(width, MinWrapWidth), MaxAutoWrapWidth)
}

// ConnectionWrapWidth is the width of a connection that may or may not have
// a user logged in: that user's WrapWidth (which applies set linewidth) when
// there is one, else LoginWrapWidth. For text sent to one connection as it
// leaves (the goodbye on quit or on a duplicate login).
func ConnectionWrapWidth(id connections.ConnectionId) int {
	if u := GetByConnectionId(id); u != nil {
		return u.WrapWidth()
	}
	return LoginWrapWidth(id)
}

// StoreAddressCols takes the raw cols value of a web client's websocket
// address (/ws?cols=57; empty when absent), stores it as StoreReportedCols
// does when it is a whole number, and returns the width to draw the connect
// splash at: the stored width, or connections.DefaultScreenWidth when the
// value is missing or bad. The caller checks that the connection is a
// websocket.
func StoreAddressCols(id connections.ConnectionId, raw string) int {
	cols, ok := ParseReportedCols(raw)
	if !ok {
		return connections.DefaultScreenWidth
	}
	return StoreReportedCols(id, cols)
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
// client sending a raw telnet screen-size report.
//
// Every other session wraps at GetLineWidth (80, or the saved LineWidth,
// which may be wider on telnet), narrowed to the width its client reported
// when that is narrower, and never below MinWrapWidth for a report (#922: a
// 36-column phone client otherwise broke words mid-line). A wider report
// never widens it. A nil receiver gets GetLineWidth.
func (u *UserRecord) WrapWidth() int {
	if u == nil {
		return u.GetLineWidth()
	}
	if !connections.IsWebsocket(u.connectionId) {
		width := u.GetLineWidth()
		if cols, ok := u.reportedScreenWidth(); ok {
			width = min(width, max(cols, MinWrapWidth))
		}
		return width
	}
	width := min(u.ClientSettings().Display.GetScreenWidth(), MaxAutoWrapWidth)
	if u.LineWidth > 0 {
		width = min(width, u.LineWidth)
	}
	return max(width, MinWrapWidth)
}
