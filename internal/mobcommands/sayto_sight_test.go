package mobcommands

import (
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
