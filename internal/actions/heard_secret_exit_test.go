package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 16: a shout heard next door through a secret exit names
// the exit only to a listener who has found it; any other hears it "from
// nearby" and learns nothing. Player and mob shouts alike.
func TestShout_NextDoorThroughASecretExitReadsNearby(t *testing.T) {
	for name, player := range map[string]bool{"player": true, "mob": false} {
		t.Run(name, func(t *testing.T) {
			sc := newSpeechScene(t)
			south := sc.next.Exits["south"]
			south.Secret = true
			sc.next.Exits["south"] = south
			require.Equal(t, []int{speechNextId}, sc.next.GetPlayers(), "fixture: Nell alone next door")
			nell := users.GetByUserId(speechNextId).Character

			// heard is what Nell reads of the shout next door, whichever lane
			// it takes (a player's rides one RoomId message, a mob's goes per
			// user).
			// walkInRead is what a player who walked in after the shout was
			// queued would be addressed by (D43): never the exit's name.
			var walkInRead []string
			heard := func() []string {
				var got []string
				walkInRead = nil
				for _, m := range events.DrainQueuedRoomMessagesForTest(speechNextRoom) {
					if m.AddressedTo(speechNextId) {
						got = append(got, strings.TrimSpace(speechTag.ReplaceAllString(m.Text, "")))
					}
					if m.AddressedTo(98899) {
						walkInRead = append(walkInRead, m.Text)
					}
				}
				for _, m := range events.DrainQueuedMessageEventsForTest(speechNextId) {
					got = append(got, strings.TrimSpace(speechTag.ReplaceAllString(m.Text, "")))
				}
				return got
			}

			Shout(sc.speaker(player, sc.lit), "HELP")
			require.Equal(t, []string{`Someone shouts from nearby, "HELP"`}, heard(), "a stranger to the exit")
			require.False(t, nell.KnowsSecretExit(speechNextRoom, "south"), "hearing teaches nothing")

			nell.LearnSecretExit(speechNextRoom, "south")
			sc.drain()
			Shout(sc.speaker(player, sc.lit), "HELP")
			require.Equal(t, []string{`Someone shouts from the south direction, "HELP"`}, heard(), "one who found it")
			for _, line := range walkInRead {
				require.NotContains(t, line, "south", "a walk-in is never addressed by the named line")
			}
			sc.drain()
		})
	}
}

// #781: next door with two ways back to the shouter, a plain one and a secret
// one, hears one line naming the plain exit, every time, and learns nothing.
func TestShout_TwoExitsBackGiveOneLineNamingThePlainOne(t *testing.T) {
	for name, player := range map[string]bool{"player": true, "mob": false} {
		t.Run(name, func(t *testing.T) {
			sc := newSpeechScene(t)
			sc.next.Exits["vent"] = exit.RoomExit{RoomId: speechLitRoom, Secret: true}
			t.Cleanup(func() { delete(sc.next.Exits, "vent") })
			nell := users.GetByUserId(speechNextId).Character
			for range 20 {
				Shout(sc.speaker(player, sc.lit), "HELP")
				var got []string
				for _, m := range events.DrainQueuedRoomMessagesForTest(speechNextRoom) {
					if m.AddressedTo(speechNextId) {
						got = append(got, strings.TrimSpace(speechTag.ReplaceAllString(m.Text, "")))
					}
				}
				for _, m := range events.DrainQueuedMessageEventsForTest(speechNextId) {
					got = append(got, strings.TrimSpace(speechTag.ReplaceAllString(m.Text, "")))
				}
				require.Equal(t, []string{`Someone shouts from the south direction, "HELP"`}, got)
				sc.drain()
			}
			require.False(t, nell.KnowsSecretExit(speechNextRoom, "vent"))
		})
	}
}
