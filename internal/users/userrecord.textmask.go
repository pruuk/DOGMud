package users

import "github.com/GoMudEngine/GoMud/internal/connections"

// inputMaskedLocked is InputMasked for a caller holding sessionMu.
func (u *UserRecord) inputMaskedLocked() bool {
	if u.activeMenu != nil || u.activePrompt == nil {
		return false
	}
	return u.activePrompt.GetNextQuestion().Masked()
}

// InputMasked reports whether the player's next line answers a secret
// question, such as a password (#868): the echo shows * and the line stays
// out of input history.
func (u *UserRecord) InputMasked() bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return u.inputMaskedLocked()
}

// Seams for tests: whether a connection is a web client, and how bytes reach it.
var (
	textMaskIsWebsocket = connections.IsWebsocket
	textMaskSend        = connections.SendTo
)

// sendTextMask tells a web client to switch its command box to a password
// field or back. Synchronous, as login does it, so the field has switched
// before the player starts typing.
func sendTextMask(connId connections.ConnectionId, masked bool) {
	if !textMaskIsWebsocket(connId) {
		return // telnet: the server's echo is already masked
	}
	cmd := "TEXTMASK:false"
	if masked {
		cmd = "TEXTMASK:true"
	}
	textMaskSend([]byte(cmd), connId)
}

// SyncTextMask sends the web client TEXTMASK when the masked state changed
// since the last send on this connection (#868). Called after each line is
// handled and whenever a prompt is cleared. The caller must not hold sessionMu.
func (u *UserRecord) SyncTextMask() {
	connId := u.ConnectionId()
	u.sessionMu.Lock()
	if u.textMaskConn != connId { // a new connection starts unmasked
		u.textMaskConn, u.textMaskSent = connId, false
	}
	masked := u.inputMaskedLocked()
	changed := masked != u.textMaskSent
	u.textMaskSent = masked
	u.sessionMu.Unlock()
	if changed {
		sendTextMask(connId, masked)
	}
}
