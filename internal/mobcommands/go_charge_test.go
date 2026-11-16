package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Movement parity 4b, owner ruling 2: a mob pays the player's step price,
// which is stamina alone since #401 (action points were removed).
func TestMobGo_PaysForTheStep(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := getTestMobAndRoom(t)
	require.Contains(t, room.Exits, "north", "fixture premise: room 1 has a north exit")

	mob.Character.StaminaMax.Value = 100
	mob.Character.Stamina = 100
	dest := rooms.LoadRoom(room.Exits["north"].RoomId)
	require.NotNil(t, dest)
	quote := actions.QuoteMove(actions.NewMobActorInRoom(mob, room), dest)
	require.True(t, quote.OK())
	require.Greater(t, quote.StaminaCost, 0.0, "a step costs stamina")

	handled, err := Go("north", mob, room)
	require.True(t, handled)
	require.NoError(t, err)
	require.NotEqual(t, 1, mob.Character.RoomId, "the mob walked north")

	mob.Character.RoomId = 1
	room.AddMob(mob.InstanceId)
}

// A mob that cannot pay the stamina stays put, silently.
func TestMobGo_ExhaustedMobStays(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	wholeStepCost(t)
	mob, room := getTestMobAndRoom(t)

	mob.Character.StaminaMax.Value = 100
	mob.Character.Stamina = 0

	handled, err := Go("north", mob, room)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, mob.Character.RoomId, "an exhausted mob does not move")
}
