package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: a player caster's
// fizzle, falter and bleed-out break told the room nothing, while the mob
// versions are seen by sight and heard by a reader who sees nothing. Player
// and mob casters read alike now. seedFallbackRoom: at lamp 10 user 1 (heat
// eyes) reads shapes; at lamp 0 user 1 (night eyes) reads nothing. User 2 is
// the caster.

// startPlayerTestCast puts u mid fold-cast against targetMobIds.
func startPlayerTestCast(t *testing.T, u *users.UserRecord, targetMobIds ...int) {
	t.Helper()
	u.Character.Activity = activity.NewMachine()
	require.NoError(t, u.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "sparks", FoldsNeeded: 4, TargetMobInstanceIds: targetMobIds},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	))
}

// bleedOut drops u to 0 health (Character.IsDisabled) until the test ends.
func bleedOut(t *testing.T, u *users.UserRecord) {
	t.Helper()
	was := u.Character.Health
	u.Character.Health = 0
	t.Cleanup(func() { u.Character.Health = was })
}

func TestPlayerSpellFizzle_ShapesSeeAFigure_BlindHearTheSpell(t *testing.T) {
	t.Run("shapes", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		startPlayerTestCast(t, caster, 999999) // no such mob: the target is gone

		require.True(t, handlePlayerFoldCasting(caster, caster.UserId))

		got := drainPlain(1)
		require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(users.GetByUserId(1).Character, room))
		require.Equal(t, 1, countContaining(got, "spell fizzles"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})

	t.Run("sees nothing", func(t *testing.T) {
		seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		startPlayerTestCast(t, caster, 999999)

		require.True(t, handlePlayerFoldCasting(caster, caster.UserId))

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})
}

func TestPlayerBleedOutBreak_IsSeenAndHeard(t *testing.T) {
	t.Run("shapes", func(t *testing.T) {
		seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		startPlayerTestCast(t, caster)
		bleedOut(t, caster)

		require.True(t, handlePlayerFoldCasting(caster, caster.UserId))

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "concentration breaks"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})

	t.Run("sees nothing", func(t *testing.T) {
		seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		startPlayerTestCast(t, caster)
		bleedOut(t, caster)

		require.True(t, handlePlayerFoldCasting(caster, caster.UserId))

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundChantBreaksOff), "%v", got)
	})
}

// The falter (not enough conviction to hold the fold) shares the fizzle's
// sender, as the mob's sendMobSpellFailed does for both.
func TestPlayerSpellFalter_ShapesSeeAFigure_BlindHearTheSpell(t *testing.T) {
	t.Run("shapes", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerSpellFailed(caster, room, "falters")

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "spell falters"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
		require.Empty(t, drainPlain(2), "the caster reads its own line, not the room's")
	})

	t.Run("sees nothing", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerSpellFailed(caster, room, "falters")

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
	})

	t.Run("clear sight names the caster", func(t *testing.T) {
		room := seedFallbackRoom(t, 60, heatEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerSpellFailed(caster, room, "falters")

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, caster.Character.Name+"'s spell falters."), "%v", got)
	})
}
