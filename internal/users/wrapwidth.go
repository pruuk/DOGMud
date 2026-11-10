package users

import "github.com/GoMudEngine/GoMud/internal/connections"

const (
	// MinWrapWidth is the narrowest a web session wraps. WCAG 2.2 AA reflow at
	// 320px needs 30 columns.
	MinWrapWidth = 30
	// MaxAutoWrapWidth is the widest a web session wraps: the game window
	// shows at most 80 columns.
	MaxAutoWrapWidth = 80
)

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
