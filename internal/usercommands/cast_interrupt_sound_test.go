package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: throw and throttle
// interrupting a mob's cast is a disruption, so a reader who sees nothing
// hears it. The room line travelled on the trio's visual observer line only.
// Each case sends the event with the categories production passes.
func TestCastInterruptMoveEvents_AreHeardAndSeen(t *testing.T) {
	cases := []struct {
		verb string
		cats moveCategories
	}{
		{"throw", throwCastInterruptCategories},
		{"throttle", throttleCastInterruptCategories},
	}
	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			// send lights the scene, blinds Bobrick (user 2) if asked, and
			// has Aliceia (user 1) interrupt the Skeleton's cast.
			send := func(blindBob bool) {
				alice, bob, room := speechWrapperScene(t)
				if blindBob {
					blindForSpeechTest(t, bob)
				}
				ids := moveIdentities{
					Actor:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, alice.Character.Name),
					ActorPlain: alice.Character.Name,
					Actee:      `<ansi fg="mobname">Skeleton</ansi>`,
					ActeePlain: "Skeleton",
				}
				aud := messaging.Audience{Actor: alice, ActorId: alice.UserId, ActorName: alice.Character.Name,
					ActeeName: "Skeleton", Room: room}
				sendMoveEvent(tc.verb, "player_cast_interrupt", ids, aud, tc.cats, nil)
			}

			t.Run("sees nothing", func(t *testing.T) {
				cleanup := seedAllRegistries()
				defer cleanup()
				send(true)
				require.Equal(t, []string{messaging.SoundChantBreaksOff}, speechWrapperHeard(2))
			})

			t.Run("sees clearly", func(t *testing.T) {
				cleanup := seedAllRegistries()
				defer cleanup()
				send(false)
				heard := speechWrapperHeard(2)
				require.Len(t, heard, 1, "%v", heard)
				require.Contains(t, heard[0], "Skeleton")
				require.NotContains(t, heard[0], messaging.SoundChantBreaksOff)
			})
		})
	}
}
