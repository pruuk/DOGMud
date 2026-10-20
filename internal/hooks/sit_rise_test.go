package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Sitting (#637) never meets a fight: entering combat stands a sitter (the
// CombatPhase observer in Position_SitRise.go) and being attacked stands one
// (RevealForCombat), each narrated once by the Position observer.

func sitRiseScene(t *testing.T) (*users.UserRecord, *users.UserRecord, *rooms.Room) {
	t.Helper()
	u1, u2 := users.GetByUserId(1), users.GetByUserId(2)
	require.NotNil(t, u1)
	require.NotNil(t, u2)
	require.NoError(t, u1.Character.Validate())
	require.NoError(t, u2.Character.Validate())
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(u2.Character, room))
	drainPlain(1)
	drainPlain(2)
	return u1, u2, room
}

func TestSitRise_PlayerEnteringCombatLeapsUp(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u1, _, room := sitRiseScene(t)

	require.True(t, actions.Sit(actions.NewUserActorInRoom(u1, room)))
	drainPlain(1)
	drainPlain(2)

	u1.Character.SetAggro(0, 100, characters.DefaultAttack)
	require.True(t, u1.Character.IsStanding(), "a sitter who starts a fight must stand, state %v", u1.Character.Position.State())
	require.Equal(t, []string{"You leap to your feet!"}, drainPlain(1))
	require.Equal(t, []string{"Aliceia leaps up!"}, drainPlain(2))
}

func TestSitRise_AttackedMobLeapsUp(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	_, _, room := sitRiseScene(t)

	mob := newLiveMob(t, 7811, room.RoomId, 40)
	mob.Character.Name = "Squatter"
	require.True(t, actions.Sit(actions.NewMobActorInRoom(mob, room)))
	require.True(t, mob.Character.IsSitting())
	require.Equal(t, []string{"Squatter sits down."}, drainPlain(2))

	mob.Character.RevealForCombat()
	require.True(t, mob.Character.IsStanding(), "an attacked sitter must stand, state %v", mob.Character.Position.State())
	require.Equal(t, []string{"Squatter leaps up!"}, drainPlain(2))

	// A second attack on a stander narrates nothing.
	mob.Character.RevealForCombat()
	require.Empty(t, drainPlain(2))
}

// Standing by command is narrated by the command, not by the combat-rise
// observer: one line each, never "leaps up".
func TestSitRise_StandCommandIsNotACombatRise(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u1, _, room := sitRiseScene(t)

	actor := actions.NewUserActorInRoom(u1, room)
	require.True(t, actions.Sit(actor))
	drainPlain(1)
	drainPlain(2)
	require.True(t, actions.StandFromSitting(actor))
	require.Equal(t, []string{"You stand up."}, drainPlain(1))
	require.Equal(t, []string{"Aliceia stands up."}, drainPlain(2))
}
