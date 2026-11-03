package guards

import (
	"os"
	"strings"
	"testing"
)

// Web players' input reaches other players' screens through say and chat, so
// control characters are removed from it. The cleaner has to run
// after TelnetIACHandler (GMCP frames are binary) and before the login prompt,
// and the world has to read the cleaned text, never the raw frame.
func TestWebsocketInputIsCleanedBeforeUse(t *testing.T) {
	src, err := os.ReadFile(`main.go`)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")

	start := strings.Index(text, `func HandleWebSocketConnection(`)
	if start < 0 {
		t.Fatal(`HandleWebSocketConnection not found in main.go`)
	}
	body := text[start:]
	if end := strings.Index(body, "\n}\n"); end >= 0 {
		body = body[:end]
	}

	iac := strings.Index(body, `inputhandlers.TelnetIACHandler)`)
	cleaner := strings.Index(body, `inputhandlers.WebsocketCleanserHandler)`)
	login := strings.Index(body, `"LoginPromptHandler", loginHandler)`)
	if iac < 0 || login < 0 {
		t.Fatalf(`handler chain markers not found (iac %d, login %d)`, iac, login)
	}
	if cleaner < iac || cleaner > login {
		t.Errorf(`WebsocketCleanserHandler must sit between the IAC handler and the login prompt (iac %d, cleaner %d, login %d)`, iac, cleaner, login)
	}
	if strings.Contains(body, `InputText: string(message)`) {
		t.Error(`the world reads the raw websocket frame`)
	}
	if !strings.Contains(body, `InputText: string(clientInput.Buffer)`) {
		t.Error(`the world should read the cleaned buffer`)
	}
}
