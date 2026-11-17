package term

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMTTS(t *testing.T) {
	for _, tt := range []struct {
		reply string
		bits  int
		ok    bool
	}{
		{`MTTS 137`, 137, true},
		{`MTTS 64`, 64, true},
		{`MTTS 0`, 0, true},
		{`MTTS -1`, 0, false},
		{`MTTS x`, 0, false},
		{`MUDLET`, 0, false},
		{`XTERM-256COLOR`, 0, false},
		{``, 0, false},
	} {
		bits, ok := ParseMTTS(tt.reply)
		require.Equal(t, tt.ok, ok, tt.reply)
		require.Equal(t, tt.bits, bits, tt.reply)
	}
}

func TestTermTypeIsMatchesAReply(t *testing.T) {
	in := append([]byte{TELNET_IAC, TELNET_SB, TELNET_OPT_TERM_TYPE, TELNET_TTYPE_IS}, []byte(`MTTS 73`)...)
	in = append(in, TELNET_IAC, TELNET_SE)
	ok, payload := Matches(in, TelnetTermTypeIs)
	require.True(t, ok)
	require.Equal(t, `MTTS 73`, string(payload))
}
