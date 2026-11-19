package inputhandlers

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/stretchr/testify/require"
)

// #922: the echo handler's real send path is connections.SendEcho, so the
// connection's prompt record follows what the player types: characters
// extend the prompt, and Enter ends it (review finding 1, where the reply
// erased the player's own command).
func TestEchoInputHandler_EchoFollowsThePromptRecord(t *testing.T) {
	server, client := net.Pipe()
	cd := connections.Add(server, nil)
	id := cd.ConnectionId()
	t.Cleanup(func() { _ = connections.Remove(id); _ = client.Close() })
	connections.UpdateClientSettings(id, func(cs *connections.ClientSettings) {
		cs.Display.ScreenWidth = 36
		cs.NAWSReported = true
	})

	var mu sync.Mutex
	var screen bytes.Buffer
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := client.Read(chunk)
			mu.Lock()
			screen.Write(chunk[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	rowsBefore := func(marker string) int {
		var text string
		require.Eventually(t, func() bool {
			mu.Lock()
			text = screen.String()
			mu.Unlock()
			return strings.Contains(text, marker)
		}, 2*time.Second, 5*time.Millisecond)
		before := text[:strings.Index(text, marker)]
		rows := 1
		for strings.HasSuffix(before, "\x1b[A\x1b[2K") {
			before = strings.TrimSuffix(before, "\x1b[A\x1b[2K")
			rows++
		}
		return rows
	}

	// A 30-column prompt, then "look north" typed: 40 columns, two rows.
	connections.SendPrompt(id, []byte(strings.Repeat("p", 30)), 30)
	EchoInputHandler(&connections.ClientInput{ConnectionId: id, DataIn: []byte("look north")}, nil)
	connections.SendReplacingPrompt(id, []byte("<M1>a line\n"))
	require.Equal(t, 2, rowsBefore("<M1>"), "typed characters extend the prompt")

	// The same, then Enter: the prompt and command are above the cursor now.
	connections.SendPrompt(id, []byte(strings.Repeat("p", 30)), 30)
	EchoInputHandler(&connections.ClientInput{ConnectionId: id, DataIn: []byte("look north"), EnterPressed: true}, nil)
	connections.SendReplacingPrompt(id, []byte("<M2>the reply\n"))
	require.Equal(t, 1, rowsBefore("<M2>"), "after Enter the reply clears one row only")
}
