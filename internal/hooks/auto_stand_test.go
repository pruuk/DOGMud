package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #398 through the real round ticks. Both fixtures are knocked down with a
// MinRecoveryRounds far past the knob, so no recovery contest ever runs and
// the auto-stand guard is the only way up: the result is deterministic, and
// "stands on round N" can only mean the guard fired. Players and mobs share
// the rule (owner 2026-10-19), so each seat has its own test.

func pinAutoStandRoundsForHooks(t *testing.T, rounds int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.ProneAutoStandRounds = configs.ConfigInt(rounds)
	configs.SetConfigForTest(t, cfg)
}

func TestUserRoundTick_AutoStandAfterExactlyNRounds(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	pinAutoStandRoundsForHooks(t, 5)

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	require.NoError(t, u.Character.Validate())
	require.NoError(t, u.Character.Position.TransitionToProne(position.ProneData{MinRecoveryRounds: 100},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward}))
	drainPlain(u.UserId)

	for round := uint64(1); round < 5; round++ {
		UserRoundTick(events.NewRound{RoundNumber: round})
		require.True(t, u.Character.IsProne(), "round %d: the player stood before ProneAutoStandRounds (5)", round)
	}
	assert.Equal(t, 0, countContaining(drainPlain(u.UserId), "scramble to your feet"),
		"no stand line while the player is still down")

	UserRoundTick(events.NewRound{RoundNumber: 5})
	require.True(t, u.Character.IsStanding(), "round 5: the guard must stand the player")
	assert.Equal(t, 1, countContaining(drainPlain(u.UserId), "You scramble to your feet!"),
		"the forced stand prints the won-recovery line to the stander")
}

func TestMobProneRecovery_AutoStandAfterExactlyNRounds(t *testing.T) {
	pinAutoStandRoundsForHooks(t, 5)

	mob := newLiveMob(t, 7801, 4242, 40)
	require.NoError(t, mob.Character.Position.TransitionToProne(position.ProneData{MinRecoveryRounds: 100},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward}))

	for round := 1; round < 5; round++ {
		tickMobProneRecovery(mob)
		require.True(t, mob.Character.IsProne(), "round %d: the mob stood before ProneAutoStandRounds (5)", round)
	}
	tickMobProneRecovery(mob)
	require.True(t, mob.Character.IsStanding(), "round 5: the guard must stand the mob")
}

func TestMobProneRecovery_AutoStandZeroDisables(t *testing.T) {
	pinAutoStandRoundsForHooks(t, 0)

	mob := newLiveMob(t, 7802, 4242, 40)
	require.NoError(t, mob.Character.Position.TransitionToProne(position.ProneData{MinRecoveryRounds: 100},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward}))

	for round := 1; round <= 20; round++ {
		tickMobProneRecovery(mob)
	}
	require.True(t, mob.Character.IsProne(), "ProneAutoStandRounds 0 must leave the mob down")
}
