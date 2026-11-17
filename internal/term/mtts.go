package term

import (
	"strconv"
	"strings"
)

// Terminal type subnegotiation (RFC 1091) and MTTS, the MUD Terminal Type
// Standard (https://tintin.mudhalla.net/protocols/mtts/). A client cycles
// its replies: its name, its terminal type, then "MTTS <bits>", then it
// repeats the last one.
const (
	// TELNET_TTYPE_IS starts a client's terminal type reply.
	TELNET_TTYPE_IS IACByte = 0
	// TELNET_TTYPE_SEND asks the client for its next terminal type.
	TELNET_TTYPE_SEND IACByte = 1
	// MTTSScreenReader is the MTTS bit a client sets when its player uses a
	// screen reader.
	MTTSScreenReader = 64
)

var (
	// TelnetWillTermType is the client agreeing to report its terminal type.
	TelnetWillTermType = TerminalCommand{[]byte{TELNET_IAC, TELNET_WILL, TELNET_OPT_TERM_TYPE}, []byte{}}
	// TelnetWontTermType is the client declining to report it.
	TelnetWontTermType = TerminalCommand{[]byte{TELNET_IAC, TELNET_WONT, TELNET_OPT_TERM_TYPE}, []byte{}}
	// TelnetTermTypeSend asks for the next terminal type in the cycle.
	TelnetTermTypeSend = TerminalCommand{[]byte{TELNET_IAC, TELNET_SB, TELNET_OPT_TERM_TYPE, TELNET_TTYPE_SEND, TELNET_IAC, TELNET_SE}, []byte{}}
	// TelnetTermTypeIs is a reply: IAC SB TTYPE IS <text> IAC SE.
	TelnetTermTypeIs = TerminalCommand{[]byte{TELNET_IAC, TELNET_SB, TELNET_OPT_TERM_TYPE, TELNET_TTYPE_IS}, []byte{TELNET_IAC, TELNET_SE}}
)

// ParseMTTS reads an MTTS reply ("MTTS 137") and returns its bit field.
// Any other reply, or a negative or non-numeric value, reports false.
func ParseMTTS(reply string) (int, bool) {
	rest, ok := strings.CutPrefix(reply, `MTTS `)
	if !ok {
		return 0, false
	}
	bits, err := strconv.Atoi(strings.TrimSpace(rest))
	if err != nil || bits < 0 {
		return 0, false
	}
	return bits, true
}
