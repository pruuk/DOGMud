package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A mob's whisper reaches the one player it names and no one else, framed
// the way a player's whisper is so it stands out from combat text (quest 28,
// owner ruling 2026-10-17). mobSpeechRoom blinds Bobrick (user 2); Aliceia
// (user 1) sees clearly.

func TestMobWhisper_ReachesOnlyTheAddressee(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := Whisper("@1 hold steady now", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`*** Skeleton whispers to you, "hold steady now" ***`}, mobSpeechHeard(1))
	require.Empty(t, mobSpeechHeard(2), "a whisper is for its addressee alone")
}

func TestMobWhisper_BlindedAddresseeHearsNoSpeakerName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := Whisper("@2 hold steady now", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`*** Someone whispers to you, "hold steady now" ***`}, mobSpeechHeard(2))
	require.Empty(t, mobSpeechHeard(1))
}

// NPC lines are authored content and reach a deafened player (ruling 6).
func TestMobWhisper_ReachesADeafenedPlayer(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)
	users.GetByUserId(1).Deafened = true

	_, err := Whisper("@1 hold steady now", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`*** Skeleton whispers to you, "hold steady now" ***`}, mobSpeechHeard(1))
}

// No target, or a target that is not a player here, says nothing.
func TestMobWhisper_NoAddresseeIsSilent(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	for _, rest := range []string{"", "@1", "@99 hello", "merchant hello"} {
		_, err := Whisper(rest, mob, room)
		require.NoError(t, err)
		require.Empty(t, mobSpeechHeard(1), "rest %q", rest)
		require.Empty(t, mobSpeechHeard(2), "rest %q", rest)
	}
}

// A long whisper wraps at 80 columns, as a player's whisper does.
func TestMobWhisper_WrapsAt80(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	long := "Now the last lesson, the one your instincts already gave you. This fight " +
		"cannot be won. Flee breaks you out of a fight. Type flee to break away and live."
	_, err := Whisper("@1 "+long, mob, room)
	require.NoError(t, err)
	heard := mobSpeechHeard(1)
	require.Len(t, heard, 1)
	lines := strings.Split(heard[0], "\n")
	require.Greater(t, len(lines), 1, "a long whisper must wrap")
	for _, l := range lines {
		require.LessOrEqual(t, len(strings.TrimRight(l, "\r")), 80, "line %q", l)
	}
}

func TestMobWhisper_IsRegistered(t *testing.T) {
	found := false
	for _, name := range GetAllMobCommands() {
		if name == "whisper" {
			found = true
		}
	}
	require.True(t, found, "quest npc_say lines with whisper: true issue the mob whisper command")
}
