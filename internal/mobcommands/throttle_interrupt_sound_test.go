package mobcommands

import (
	"fmt"
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4: a throttle interrupting a cast is a disruption, so
// a reader who sees nothing hears it. Mobs and players share one pool per
// event (M6 slice 2), so a mob's throttle interrupt reaches the room as a
// player's does, sound line included. Mirrors the player side's
// TestCastInterruptMoveEvents_AreHeardAndSeen.
func TestMobThrottleCastInterrupt_IsHeardAndSeen(t *testing.T) {
	if err := movenarration.LoadFrom("../../_datafiles/world/dogmud/narration/special-moves"); err != nil {
		t.Fatalf("load special-move narration: %v", err)
	}
	// send lights room 1 exactly, blinds Bobrick (user 2) if asked, and has
	// the Skeleton (mob 100) choke Aliceia's (user 1) cast, with the
	// arguments production passes in throttle.go.
	send := func(blindBob bool) {
		mob, room := getTestMobAndRoom(t)
		room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
		alice, bob := users.GetByUserId(1), users.GetByUserId(2)
		bob.Character.Perception = characters.New().Perception
		if blindBob {
			require.NoError(t, bob.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
		}
		events.DrainQueuedMessagesForTest(1)
		events.DrainQueuedMessagesForTest(2)

		mobName := mob.Character.Name
		ids := moveIdentities{
			Actor:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, mobName),
			ActorPlain: mobName,
			Actee:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, alice.Character.Name),
			ActeePlain: alice.Character.Name,
		}
		aud := messaging.Audience{ActorName: mobName, Actee: alice, ActeeId: alice.UserId,
			ActeeName: alice.Character.Name, Room: room}
		sendMoveEvent("throttle", "cast_interrupt", ids, aud, messaging.CategorySpellDisruption, nil, messaging.SoundChantBreaksOff)
	}

	t.Run("sees nothing", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		send(true)
		require.Equal(t, []string{messaging.SoundChantBreaksOff}, mobSpeechHeard(2))
	})

	t.Run("sees clearly", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		send(false)
		heard := mobSpeechHeard(2)
		require.Len(t, heard, 1, "%v", heard)
		require.Contains(t, heard[0], "Skeleton")
		require.NotContains(t, heard[0], messaging.SoundChantBreaksOff)
	})

	// The scene above proves what the call delivers; this proves throttle.go
	// makes that call, sound included.
	t.Run("throttle.go passes the sound", func(t *testing.T) {
		raw, err := os.ReadFile("throttle.go")
		require.NoError(t, err)
		require.Contains(t, string(raw),
			`sendMoveEvent("throttle", "cast_interrupt", ids, aud, messaging.CategorySpellDisruption, nil, messaging.SoundChantBreaksOff)`)
	})
}
