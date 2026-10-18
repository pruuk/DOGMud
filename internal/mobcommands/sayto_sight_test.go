package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A mob speaking to someone in particular is speech like any other (sight
// gates 5b ruling 3): every listener hears the words, and each name in the
// line, the speaker's and the addressee's, follows that listener's sight.
// mobSpeechRoom blinds Bobrick (user 2); Aliceia (user 1) sees clearly.

func TestMobSayTo_BlindedBystanderHearsNoNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := SayTo("alice hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Skeleton says to you, "hello there"`}, mobSpeechHeard(1))
	require.Equal(t, []string{`Someone says to someone, "hello there"`}, mobSpeechHeard(2))
}

func TestMobSayTo_BlindedAddresseeHearsNoSpeakerName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := SayTo("bob hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone says to you, "hello there"`}, mobSpeechHeard(2))
	require.Equal(t, []string{`Skeleton says to Bobrick, "hello there"`}, mobSpeechHeard(1))
}

func TestMobSayTo_MobAddresseeHiddenFromTheBlind(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := SayTo("merchant hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Skeleton says to Merchant, "hello there"`}, mobSpeechHeard(1))
	require.Equal(t, []string{`Someone says to someone, "hello there"`}, mobSpeechHeard(2))
}

func TestMobSayToOnly_BlindedAddresseeHearsNoSpeakerName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := SayToOnly("bob a secret", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone says to you, "a secret"`}, mobSpeechHeard(2))
	require.Empty(t, mobSpeechHeard(1), "sayto-only reaches the addressee alone")
}

// An addressed line wraps at 80 columns the way a plain NPC say does
// (actions.FormatSayText): Cleric Hadwen's rite (9100) speaks long lines
// through sayto, one to each asker, since the 2026-10-17 newbie-duo fix.
func TestMobSayTo_LongLineWrapsAt80(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	long := "the pool answers first, it is older than the town and more patient than I am, so look into the water and wait"
	_, err := SayTo("alice "+long, mob, room)
	require.NoError(t, err)
	_, err = SayTo("bob "+long, mob, room)
	require.NoError(t, err)
	for _, uid := range []int{1, 2} {
		heard := mobSpeechHeard(uid)
		require.NotEmpty(t, heard)
		for _, msg := range heard {
			for _, line := range strings.Split(strings.ReplaceAll(msg, "\r", ""), "\n") {
				require.LessOrEqual(t, len(line), 80, "user %d heard an unwrapped line: %q", uid, line)
			}
		}
	}
}

func TestMobReplyTo_BlindedListenersHearNoNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)

	_, err := ReplyTo("alice some reply", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Skeleton replies to you, "some reply"`}, mobSpeechHeard(1))
	require.Equal(t, []string{`Someone replies to someone, "some reply"`}, mobSpeechHeard(2))

	_, err = ReplyTo("bob some reply", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone replies to you, "some reply"`}, mobSpeechHeard(2))

	_, err = ReplyTo("merchant some reply", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`Someone replies to someone, "some reply"`}, mobSpeechHeard(2))
}

// A shapes-only listener reads "a figure" for both parties.
func TestMobSayTo_ShapesBystanderReadsFigures(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)
	bob := users.GetByUserId(2)
	bob.Character.Perception = characters.New().Perception
	room.Lamp = rooms.LampPtr(35)
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(bob.Character, room))
	events.DrainQueuedMessagesForTest(2)

	_, err := SayTo("alice hello there", mob, room)
	require.NoError(t, err)
	require.Equal(t, []string{`A figure says to a figure, "hello there"`}, mobSpeechHeard(2))
}
