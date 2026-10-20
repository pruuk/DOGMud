package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/require"
)

// A mob sits with `sit` or `rest` and gets up with `stand` (#637), through
// the mob dispatcher. The seeded keywords carry no command aliases, so `rest`
// reaching Sit here proves the mob registry's own synonym.
// A sleeping mob that sits is refused silently and stays asleep, as a
// sleeping player is (owner ruling 2026-10-19).
func TestMobSit_SleepingMobStaysAsleep(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		15: {ConditionId: 15, Name: "Sleeping", TriggerCount: 1000000,
			Flags: []conditions.Flag{conditions.Sleeping, conditions.CancelOnAction, conditions.CancelOnDamage}},
	})()
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	require.NoError(t, mob.Character.Validate())
	require.NoError(t, mob.Character.AddCondition(15, false))
	require.True(t, mob.Character.HasConditionFlag(conditions.Sleeping))

	_, err := TryCommand("sit", "", 100)
	require.NoError(t, err)
	require.True(t, mob.Character.HasConditionFlag(conditions.Sleeping), "sit must not wake the sleeping mob")
	require.True(t, mob.Character.IsStanding(), "a sleeping mob must not sit, state %v", mob.Character.Position.State())
}

// A busy mob (casting) is refused, as a busy player is.
func TestMobSit_CastingMobIsRefused(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	require.NoError(t, mob.Character.Validate())
	mob.Character.Activity = activity.NewMachine()
	require.NoError(t, mob.Character.Activity.TransitionToCasting(activity.CastingData{SpellId: "sparks", FoldsNeeded: 4},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))

	_, err := TryCommand("sit", "", 100)
	require.NoError(t, err)
	require.True(t, mob.Character.IsStanding(), "a casting mob must not sit, state %v", mob.Character.Position.State())
}

func TestMobSitAndRestBothSeatAndStandGetsUp(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	require.NoError(t, mob.Character.Validate())

	for _, verb := range []string{"sit", "rest"} {
		handled, err := TryCommand(verb, "", 100)
		require.True(t, handled, verb)
		require.NoError(t, err, verb)
		require.True(t, mob.Character.IsSitting(), "%s must seat the mob, state %v", verb, mob.Character.Position.State())

		handled, err = TryCommand("stand", "", 100)
		require.True(t, handled)
		require.NoError(t, err)
		require.True(t, mob.Character.IsStanding(), "stand must get a sitting mob up")
	}
}
