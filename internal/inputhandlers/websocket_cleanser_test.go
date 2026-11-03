package inputhandlers

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
)

func TestWebsocketCleanserRemovesControlCharacters(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{`plain command`, `say hello`, `say hello`},
		{`accents and emoji`, `say héllo wörld 🚀`, `say héllo wörld 🚀`},
		{`seven bit escape`, "say a\x1b[31mb", `say a[31mb`},
		{`eight bit escape`, "say a\u009b31mb", `say a31mb`},
		{`line ends`, "say a\r\nb\n", `say ab`},
		{`bell and null`, "say a\x07\x00b", `say ab`},
		{`nothing left`, "\x1b\x07", ``},
		{`delete`, "say a\x7fb", `say ab`},
		{`tab is a space`, "say a\tb", `say a b`},
		// Passwords are hashed as typed and these were always legal, so
		// dropping them would lock their owners out for good. They also keep
		// words apart in chat.
		{`non-breaking space`, "pass word", "pass word"},
		{`wide space`, "say 你好　世界", "say 你好　世界"},
		{`joined emoji`, "say 👨‍👩‍👧", "say 👨‍👩‍👧"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := []byte(c.in)
			ci := &connections.ClientInput{DataIn: msg, Buffer: msg, EnterPressed: true}

			if !WebsocketCleanserHandler(ci, nil) {
				t.Fatal(`the chain must go on to the next handler`)
			}
			if string(ci.Buffer) != c.want {
				t.Errorf(`Buffer = %q, want %q`, ci.Buffer, c.want)
			}
			if string(ci.DataIn) != c.want {
				t.Errorf(`DataIn = %q, want %q`, ci.DataIn, c.want)
			}
			if !ci.EnterPressed {
				t.Error(`a websocket message is always a whole line`)
			}
		})
	}
}
