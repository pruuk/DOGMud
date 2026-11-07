package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// whisperFixture seeds two players, Kesh (9751) and Ana (9752).
func whisperFixture(t *testing.T) {
	t.Helper()
	kesh := users.NewTestUser(9751, "kesh", "Kesh", 97751)
	ana := users.NewTestUser(9752, "ana", "Ana", 97752)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9751: kesh, 9752: ana}))
}

func whisperSenders(evt events.Communication) map[int]string {
	got := map[int]string{}
	for _, d := range whisperDeliveries(evt, GMCPCommModule_Payload{Channel: `whisper`, Sender: evt.Name, Text: evt.Message}) {
		got[d.UserId] = d.Payload.Sender
	}
	return got
}

// A player's whisper reaches its target under the sender's name, and comes
// back to the sender as "You (to <target>)" so their Whisper tab shows both
// sides of the talk.
func TestWhisperDeliveries_EchoesToSender(t *testing.T) {
	whisperFixture(t)
	got := whisperSenders(events.Communication{SourceUserId: 9751, TargetUserId: 9752, CommType: `whisper`, Name: `Kesh`, Message: `hi`})
	require.Equal(t, map[int]string{9752: `Kesh`, 9751: `You (to Ana)`}, got)
}

// A mob's whisper has no player sender, so only the target gets it.
func TestWhisperDeliveries_MobWhisperReachesOnlyTarget(t *testing.T) {
	whisperFixture(t)
	got := whisperSenders(events.Communication{SourceMobInstanceId: 741, TargetUserId: 9752, CommType: `whisper`, Name: `Crier`, Message: `psst`})
	require.Equal(t, map[int]string{9752: `Crier`}, got)
}

// A whisper to yourself is delivered once, not echoed a second time.
func TestWhisperDeliveries_ToSelfOnce(t *testing.T) {
	whisperFixture(t)
	got := whisperDeliveries(events.Communication{SourceUserId: 9751, TargetUserId: 9751, CommType: `whisper`, Name: `Kesh`, Message: `note`},
		GMCPCommModule_Payload{Channel: `whisper`, Sender: `Kesh`})
	require.Len(t, got, 1)
	require.Equal(t, `Kesh`, got[0].Payload.Sender)
}

// No target, no delivery.
func TestWhisperDeliveries_NoTarget(t *testing.T) {
	require.Empty(t, whisperDeliveries(events.Communication{SourceUserId: 9751, CommType: `whisper`}, GMCPCommModule_Payload{}))
}
