package connections

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// A record written by a binary from before conn_type and strip_ansi existed
// restores as a human connection with ANSI kept, as it always did.
func TestConnectionRecord_OldRecordRestoresAsHuman(t *testing.T) {
	old := `{"connection_id":12,"state":2,"fd":7,"client_settings":{}}`
	var rec connectionRecord
	require.NoError(t, json.Unmarshal([]byte(old), &rec))
	require.Equal(t, ConnHuman, rec.ConnType)
	require.False(t, rec.StripAnsi)
	require.Equal(t, ConnHuman, rec.restoredConnType())
}

// A record from this binary carries an AI-port connection's type and ANSI
// stripping across the handoff.
func TestConnectionRecord_RoundTripsTypeAndStripAnsi(t *testing.T) {
	in := connectionRecord{ConnectionId: 12, State: LoggedIn, Fd: 7, ConnType: ConnAI, StripAnsi: true}
	b, err := json.Marshal(in)
	require.NoError(t, err)

	var out connectionRecord
	require.NoError(t, json.Unmarshal(b, &out))
	require.Equal(t, ConnAI, out.ConnType)
	require.True(t, out.StripAnsi)
	require.Equal(t, ConnAI, out.restoredConnType())
}

// A type value this binary does not know restores as a human connection.
func TestConnectionRecord_UnknownTypeRestoresAsHuman(t *testing.T) {
	var rec connectionRecord
	require.NoError(t, json.Unmarshal([]byte(`{"conn_type":7}`), &rec))
	require.Equal(t, ConnHuman, rec.restoredConnType())
}
