package inputhandlers

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/term"
)

var (
	iacHandlers = []IACHandler{}
)

type IACHandler interface {
	HandleIAC(uint64, []byte) bool
}

func AddIACHandler(h IACHandler) {
	iacHandlers = append(iacHandlers, h)
}

// maxTermTypeReplies bounds the MTTS cycle: name, terminal type, MTTS, and
// one spare for a client that repeats late.
const maxTermTypeReplies = 4

// maxTermTypeLen is the most of a terminal type reply kept for comparing
// with the next one; a reply comes from the client.
const maxTermTypeLen = 64

// recordTermType handles one terminal type reply and reports whether to ask
// for the next. An MTTS reply records whether the player uses a screen
// reader and ends the cycle. A reply equal to the last one ends it (the
// client has cycled back without MTTS). Otherwise the server asks again, at
// most maxTermTypeReplies times in all.
func recordTermType(id connections.ConnectionId, reply string) bool {
	if len(reply) > maxTermTypeLen {
		reply = reply[:maxTermTypeLen]
	}
	cs := connections.GetClientSettings(id)
	cs.TermTypeReplies++
	last := cs.TermTypeLast
	cs.TermTypeLast = reply

	askAgain := false
	if bits, ok := term.ParseMTTS(reply); ok {
		cs.MTTSReported = true
		cs.ScreenReader = bits&term.MTTSScreenReader != 0
		mudlog.Debug("Received", "type", "IAC (MTTS)", "bits", bits)
	} else {
		mudlog.Debug("Received", "type", "IAC (TTYPE)", "reply", reply)
		askAgain = reply != last && cs.TermTypeReplies < maxTermTypeReplies
	}
	connections.OverwriteClientSettings(id, cs)
	return askAgain
}

// TelnetIACHandler reads Telnet IAC commands from client input and answers
// or records them; input with no IAC command passes to the next handler.
func TelnetIACHandler(clientInput *connections.ClientInput, sharedState map[string]any) (nextHandler bool) {

	// Check for Telnet IAC commands
	// If not, pass it on to next handler
	if !term.IsTelnetCommand(clientInput.DataIn) {
		return true
	}

	// Multiple Telnet IAC's can be stacked into one send, so useful to split them out
	iacCmds := [][]byte{}

	var lastIAC int = 0
	for i, b := range clientInput.DataIn {
		if i != 0 && b == term.TELNET_IAC {
			if i < len(clientInput.DataIn)-1 && clientInput.DataIn[i+1] != term.TELNET_SE {
				iacCmds = append(iacCmds, clientInput.DataIn[lastIAC:i])
				lastIAC = i
			}
		}
	}

	//mudlog.Debug("Received", "type", "IAC (TEST)", "data", term.BytesString(clientInput.DataIn))

	if lastIAC < len(clientInput.DataIn) {
		iacCmds = append(iacCmds, clientInput.DataIn[lastIAC:])
	}

	for _, iacCmd := range iacCmds {
		// Check incoming Telnet IAC commands for anything useful...

		if len(iacHandlers) > 0 {

			handlerFound := false
			for _, h := range iacHandlers {
				if h.HandleIAC(clientInput.ConnectionId, iacCmd) {
					handlerFound = true
					break
				}
			}

			if handlerFound {
				continue
			}

		}

		if term.IsMSSPCommand(iacCmd) {
			// A crawler accepted our WILL MSSP (IAC DO MSSP) — reply once with
			// the status block. A DONT/refuse is a silent no-op.
			if ok, _ := term.Matches(iacCmd, term.MsspAccept); ok {
				if fields := gatherMSSPFields(); len(fields) > 0 {
					connections.SendTo(
						term.MsspCommand.BytesWithPayload(term.EncodeMSSPPayload(fields)),
						clientInput.ConnectionId,
					)
				}
			}
			continue
		}

		if term.IsMSPCommand(iacCmd) {

			if ok, payload := term.Matches(iacCmd, term.MspAccept); ok {
				mudlog.Debug("Received", "type", "IAC (Client-MSP Accept)", "data", term.BytesString(payload))

				cs := connections.GetClientSettings(clientInput.ConnectionId)
				cs.MSPEnabled = true
				connections.OverwriteClientSettings(clientInput.ConnectionId, cs)

				connections.SendTo(
					term.MspCommand.BytesWithPayload([]byte("!!SOUND(Off U="+configs.GetFilePathsConfig().WebCDNLocation.String()+")")),
					clientInput.ConnectionId,
				)

				continue
			}

			if ok, payload := term.Matches(iacCmd, term.MspRefuse); ok {
				mudlog.Debug("Received", "type", "IAC (Client-MSP Refuse)", "data", term.BytesString(payload))

				cs := connections.GetClientSettings(clientInput.ConnectionId)
				cs.MSPEnabled = false
				connections.OverwriteClientSettings(clientInput.ConnectionId, cs)

				continue
			}

			continue
		}

		if ok, payload := term.Matches(iacCmd, term.TelnetAcceptedChangeCharset); ok {
			mudlog.Debug("Received", "type", "IAC (TelnetAcceptedChangeCharset)", "data", term.BytesString(payload))
			continue
		}

		if ok, _ := term.Matches(iacCmd, term.TelnetRejectedChangeCharset); ok {
			mudlog.Debug("Received", "type", "IAC (TelnetRejectedChangeCharset)")
			continue
		}

		if ok, _ := term.Matches(iacCmd, term.TelnetAgreeChangeCharset); ok {
			mudlog.Debug("Received", "type", "IAC (TelnetAgreeChangeCharset)")
			connections.SendTo(
				term.TelnetCharset.BytesWithPayload([]byte(" UTF-8")),
				clientInput.ConnectionId,
			)
			continue
		}

		if ok, _ := term.Matches(iacCmd, term.TelnetDontSuppressGoAhead); ok {
			mudlog.Debug("Received", "type", "IAC (TelnetDontSuppressGoAhead)")

			cs := connections.GetClientSettings(clientInput.ConnectionId)
			cs.SendTelnetGoAhead = true
			connections.OverwriteClientSettings(clientInput.ConnectionId, cs)

			continue
		}

		// Terminal type and MTTS: the client may declare a screen reader
		// before login (connect sends IAC DO TTYPE, main.go).
		if ok, _ := term.Matches(iacCmd, term.TelnetWillTermType); ok {
			connections.SendTo(term.TelnetTermTypeSend.BytesWithPayload(nil), clientInput.ConnectionId)
			continue
		}
		if ok, _ := term.Matches(iacCmd, term.TelnetWontTermType); ok {
			continue
		}
		if ok, payload := term.Matches(iacCmd, term.TelnetTermTypeIs); ok {
			if recordTermType(clientInput.ConnectionId, string(payload)) {
				connections.SendTo(term.TelnetTermTypeSend.BytesWithPayload(nil), clientInput.ConnectionId)
			}
			continue
		}

		// Is it a screen size report?
		if ok, payload := term.Matches(iacCmd, term.TelnetScreenSizeResponse); ok {

			w, h, err := term.TelnetParseScreenSizePayload(payload)
			if err != nil {
				mudlog.Debug("Received", "type", "IAC (Screensize)", "data", term.BytesString(payload), "error", err)
			} else {
				mudlog.Debug("Received", "type", "IAC (Screensize)", "width", w, "height", h)

				if err == nil {

					cs := connections.GetClientSettings(clientInput.ConnectionId)
					cs.Display.ScreenWidth = uint32(w)
					cs.Display.ScreenHeight = uint32(h)
					connections.OverwriteClientSettings(clientInput.ConnectionId, cs)

				}

			}

			continue
		}

		// Unhanlded IAC command, log it
		mudlog.Debug("Received", "type", "IAC (Unhandled)", "size", len(clientInput.DataIn), "data", term.TelnetCommandToString(iacCmd))

	}

	// We handled it, so don't pass it on
	return false
}
