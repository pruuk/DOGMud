package inputhandlers

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/term"
)

// maskSeams sets whether the connection's next line is a secret answer and
// captures what the echo handler sends (#868).
func maskSeams(t *testing.T, masked bool) *[]byte {
	t.Helper()
	var sent []byte
	oldMasked, oldSend := inputMasked, echoSend
	inputMasked = func(connections.ConnectionId) bool { return masked }
	echoSend = func(b []byte, _ ...connections.ConnectionId) { sent = append(sent, b...) }
	t.Cleanup(func() { inputMasked, echoSend = oldMasked, oldSend })
	return &sent
}

func TestEchoInputHandler_MaskedKeystrokeAndLine(t *testing.T) {
	cases := []struct {
		name string
		ci   connections.ClientInput
		want string
	}{
		{"telnet keystroke", connections.ClientInput{DataIn: []byte("a")}, "*"},
		{"web line", connections.ClientInput{DataIn: []byte("hunter2x"), Buffer: []byte("hunter2x"), EnterPressed: true}, "********" + string(term.CRLF)},
		{"multibyte letter is one star", connections.ClientInput{DataIn: []byte("pé")}, "**"},
		{"enter alone", connections.ClientInput{EnterPressed: true}, string(term.CRLF)},
		// The cleanser already erased on screen and emptied DataIn.
		{"backspace sends nothing here", connections.ClientInput{BSPressed: true}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sent := maskSeams(t, true)
			ci := c.ci
			EchoInputHandler(&ci, nil)
			if string(*sent) != c.want {
				t.Errorf("sent %q, want %q", *sent, c.want)
			}
		})
	}
}

func TestEchoInputHandler_UnmaskedEchoesRaw(t *testing.T) {
	sent := maskSeams(t, false)
	ci := connections.ClientInput{DataIn: []byte("look"), Buffer: []byte("look"), EnterPressed: true}
	EchoInputHandler(&ci, nil)
	if want := "look" + string(term.CRLF); string(*sent) != want {
		t.Errorf("sent %q, want %q", *sent, want)
	}
}

func TestHistoryInputHandler_SkipsMaskedLines(t *testing.T) {
	for _, masked := range []bool{true, false} {
		maskSeams(t, masked)
		ci := connections.ClientInput{Buffer: []byte("secret"), EnterPressed: true}
		HistoryInputHandler(&ci, nil)
		got := string(ci.History.Get())
		if masked && got != "" {
			t.Errorf("a masked line was added to history: %q", got)
		}
		if !masked && got != "secret" {
			t.Errorf("a plain line must be added to history, got %q", got)
		}
	}
}
