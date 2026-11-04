package inputhandlers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/term"
)

// CleanserInputHandler's job is to remove any bad characters from the input stream
// before passing it down the chain.
// For this reason, it's important it happen before other text processing handlers
func CleanserInputHandler(clientInput *connections.ClientInput, sharedState map[string]any) (nextHandler bool) {

	if len(clientInput.DataIn) < 1 {
		return true
	}

	// backspace
	dIn := clientInput.DataIn[len(clientInput.DataIn)-1]

	if dIn == term.ASCII_DELETE || dIn == term.ASCII_BACKSPACE {

		clientInput.BSPressed = true

		//connections.SendTo([]byte(term.AnsiMoveCursorBackward.String()+" "+term.AnsiMoveCursorBackward.String()), connDetails.UniqueId())
		// send backspace, space, backspace
		if len(clientInput.Buffer) > 0 {
			connections.SendTo([]byte{term.ASCII_BACKSPACE, term.ASCII_SPACE, term.ASCII_BACKSPACE}, clientInput.ConnectionId)

			// Handle UTF-8 properly by removing the last complete character (rune)
			bufferStr := string(clientInput.Buffer)
			if len(bufferStr) > 0 {
				// Find the start of the last rune
				_, size := utf8.DecodeLastRune(clientInput.Buffer)
				if size > 0 {
					clientInput.Buffer = clientInput.Buffer[:len(clientInput.Buffer)-size]
				}
			}
		}
		clientInput.DataIn = clientInput.DataIn[:len(clientInput.DataIn)-1]
		return true
	}

	if dIn == term.ASCII_TAB {
		clientInput.TabPressed = true
	} else {
		// Check if the last byte is a CR or LF or NULL
		if dIn <= term.ASCII_CR {
			if clientInput.DataIn[len(clientInput.DataIn)-1] == term.ASCII_NULL || clientInput.DataIn[len(clientInput.DataIn)-1] == term.ASCII_LF || clientInput.DataIn[len(clientInput.DataIn)-1] == term.ASCII_CR {
				clientInput.EnterPressed = true
			}
		}
	}

	// Remove non printing chars
	//clientInput.DataIn = trimNonPrintingBytes(clientInput.DataIn)

	clientInput.DataIn = []byte(strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, string(clientInput.DataIn)))

	clientInput.DataIn = trimToLineRoom(clientInput.DataIn, len(clientInput.Buffer))

	// Add all input to the currentBuffer
	clientInput.Buffer = append(clientInput.Buffer, clientInput.DataIn...)

	return true
}

// trimToLineRoom cuts data to what still fits in a line already holding
// bufferLen bytes, on a character boundary. Enter and backspace are read before
// this, so a full line can still be sent or shortened. It trims DataIn itself
// because the echo handler prints DataIn, and the player should see what the
// line really holds.
func trimToLineRoom(data []byte, bufferLen int) []byte {
	room := connections.MaxInputLineBytes - bufferLen
	if room < 0 {
		room = 0
	}
	if len(data) <= room {
		return data
	}
	for room > 0 && !utf8.RuneStart(data[room]) {
		room--
	}
	return data[:room]
}

// WebsocketCleanserHandler removes control characters from websocket input, so
// a line cannot drive another player's terminal. A websocket message is a whole
// line, so there are no keystrokes to handle. It runs after TelnetIACHandler,
// which takes the binary GMCP frames this would otherwise corrupt.
//
// It is narrower than the telnet rule on purpose. Web passwords were always
// hashed as typed, including non-breaking spaces and joined emoji, so dropping
// those would lock their owners out; they also keep words apart in chat.
func WebsocketCleanserHandler(clientInput *connections.ClientInput, sharedState map[string]any) (nextHandler bool) {
	clientInput.DataIn = withoutControlChars(clientInput.DataIn)
	clientInput.Buffer = withoutControlChars(clientInput.Buffer)
	return true
}

// withoutControlChars drops control characters and turns a tab into a space.
func withoutControlChars(b []byte) []byte {
	return []byte(strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, string(b)))
}

// Trims non printing bytes and SPACE from front/back of a byte slice
func trimNonPrintingBytes(b []byte) []byte {
	start := 0
	for ; start < len(b); start++ {
		c := b[start]
		if c > 31 && c < 127 {
			break
		}
	}

	stop := len(b)
	for ; stop > start; stop-- {
		c := b[stop-1]
		if c > 31 && c < 127 {
			break
		}
	}

	return b[start:stop]
}
