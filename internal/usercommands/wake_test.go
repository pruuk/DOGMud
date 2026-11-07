package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/stretchr/testify/require"
)

// Spec: wake <name> wakes a sleeping NPC in the room.
func TestWakeByNameWakesTheSleeper(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	_, err := Wake(`marn`, kesh, shop, 0)
	require.NoError(t, err)
	require.False(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}

// Bare wake does nothing more: the dispatcher already woke a sleeping player.
func TestBareWakeIsHarmless(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	_, err := Wake(``, kesh, shop, 0)
	require.NoError(t, err)
	require.True(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}

// A name that matches nobody in the room changes nothing.
func TestWakeUnknownNameChangesNothing(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	_, err := Wake(`nobody`, kesh, shop, 0)
	require.NoError(t, err)
	require.True(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}
