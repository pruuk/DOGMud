package inputhandlers

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Seams for tests (#868): whether the connection's next line answers a secret
// question, and how the echo reaches the client.
var (
	inputMasked = func(id connections.ConnectionId) bool {
		u := users.GetByConnectionId(id)
		return u != nil && u.InputMasked()
	}
	echoSend = connections.SendTo
)

// maskEcho returns what to echo for data while the answer is a secret (#868):
// one * per printable character (a multi-byte letter shows one), other bytes
// unchanged, as the login handler does.
func maskEcho(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for _, b := range data {
		switch {
		case b >= 0x80 && b < 0xC0: // UTF-8 continuation byte
		case b >= 32 && b != 127:
			out = append(out, '*')
		default:
			out = append(out, b)
		}
	}
	return out
}

func EchoInputHandler(clientInput *connections.ClientInput, sharedState map[string]any) (nextHandler bool) {

	// Web inventory-panel commands target an exact item instance via an
	// "@<uuid>" handle token (e.g. `look @1-000000000000000-01-00000000000000`).
	// The web client does NOT echo locally and relies on the server echo, so
	// for these silent panel actions we suppress the server-side echo of the
	// command line (and its trailing newline) — the player should see ONLY the
	// command's result in the feed, never the raw handle. Normal typed commands
	// keep their echo untouched. See lineTargetsItemHandle for why this can
	// never catch a normal command / emote / chat line.
	if len(clientInput.DataIn) > 0 && lineTargetsItemHandle(string(clientInput.DataIn)) {
		// Silent: skip the line echo AND the Enter/CRLF echo, but still let the
		// command continue down the chain to be processed.
		return true
	}

	// If no actual input, for now just do/change nothing
	if len(clientInput.DataIn) > 0 {
		// echo it back; a secret answer (password prompt) shows as * (#868)
		if inputMasked(clientInput.ConnectionId) {
			echoSend(maskEcho(clientInput.DataIn), clientInput.ConnectionId)
		} else {
			echoSend(clientInput.DataIn, clientInput.ConnectionId)
		}
	}

	// if they didn't hit enter, just keep buffering, go next.
	if !clientInput.EnterPressed {
		return false
	}

	// Echo back their Enter press
	echoSend(term.CRLF, clientInput.ConnectionId)

	return true
}

// lineTargetsItemHandle reports whether the given input line contains an
// item-handle target token: a whitespace-delimited token that begins with the
// item-handle sigil ("@") followed by a UUID-shaped string.
//
// This is deliberately CONSERVATIVE so it can never suppress a normal command,
// emote, or chat line. The engine's item UUID String() form is exactly 35
// characters of hex digits and hyphens (e.g. "1-000000000000000-01-0000...").
// We require the post-sigil portion to be EXACTLY that length and contain only
// [0-9a-f-]. A line like "@bob hi there" or "say @everyone" can never match
// (wrong length / non-hex characters), so this only ever fires for the web
// panel's machine-generated "@<uuid>" tokens.
//
// For telnet clients DataIn is a single keystroke (per-character echo), so it
// is never 35+ chars and never matches — telnet echo is unaffected by design.
func lineTargetsItemHandle(line string) bool {
	for tok := range strings.FieldsSeq(line) {
		if !strings.HasPrefix(tok, characters.ItemHandleSigil) {
			continue
		}
		handle := strings.TrimPrefix(tok, characters.ItemHandleSigil)
		if isUUIDShaped(handle) {
			return true
		}
	}
	return false
}

// isUUIDShaped reports whether s has the exact shape of an engine item UUID
// string: 35 characters drawn only from lowercase hex digits and hyphens.
const itemUUIDStringLen = 35

func isUUIDShaped(s string) bool {
	if len(s) != itemUUIDStringLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !isHex && c != '-' {
			return false
		}
	}
	return true
}
