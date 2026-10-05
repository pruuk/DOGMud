package inputhandlers

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/stretchr/testify/assert"
)

// TestTelnetIACHandlerShortScreenSizeReport sends a NAWS report cut short after
// three bytes. It used to index past the payload and panic on the connection
// goroutine; DOGMud's recover in handleTelnetConnection kept the server up but
// still dropped the connection. Ported from upstream GoMud #647.
func TestTelnetIACHandlerShortScreenSizeReport(t *testing.T) {
	clientInput := &connections.ClientInput{
		ConnectionId: 1,
		DataIn:       []byte{term.TELNET_IAC, term.TELNET_SB, term.TELNET_OPT_NAWS, 0, 80, 0},
	}

	assert.NotPanics(t, func() {
		TelnetIACHandler(clientInput, map[string]any{})
	})
}
