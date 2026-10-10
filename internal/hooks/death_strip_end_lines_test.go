package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Records the death strip expired must not narrate their end lines after the
// respawn.
//
// The playtest (run 7d0dad99c4709fc0): after "Darkness swallows you. When you
// open your eyes, you are somewhere safe." the player read "Your wounds stop
// bleeding." in the Mending Hut. The death strip only EXPIRES records; the
// next NewTurn prune removed them and narrated each end line to wherever the
// player then stood.
//
// The death cause is read by an Alive -> Dead observer registered AFTER the
// cascade, on purpose. It calls deathCauseFor, exactly as the death
// announcement does, at the same transition. That is the order in which a
// strip that also pruned would blind deathCauseFor (it reads the held Bleeding
// record by id), so the "bleeding out" assertion pins that the silent prune
// waits until every Alive -> Dead observer has run, whatever order they were
// registered in. (The real announcement runs too, from the fixture's
// production wiring, which registers it before the cascade.)
func TestDeathStrip_ExpiredRecordsDoNotNarrateAfterRespawn(t *testing.T) {
	u := setupConditionAfterDeath(t)
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	cause := ""
	u.Character.Life.Inner().AfterTransition("test_death_cause",
		func(from, to life.State, _ state.TransitionReason) {
			if from == life.Alive && to == life.Dead {
				cause = deathCauseFor(u.Character)
			}
		})

	// A one-trigger bleed that kills, and a narrated shield that would have
	// lasted. Both end lines must stay silent.
	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 1, -5, "claws"))
	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 10, 3, "spell"))
	u.Character.Health = 1

	UserRoundTick(events.NewRound{RoundNumber: 1})
	require.LessOrEqual(t, u.Character.Health, 0, "the bleed tick must take the last point")
	// Isolate the held-record read: the LastTickCause fallback is pinned by
	// TestPin_BleedTickKillsAndNamesTheCause, and would mask a record pruned
	// before the announcement ran.
	u.Character.LastTickCause = ""

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1, "the bleed tick must queue the death")
	epochBefore := u.Character.LifeEpoch
	RouteAttributedDeath(died[0])
	require.True(t, u.Character.IsAlive(), "precondition: the player respawned")
	require.Equal(t, epochBefore+1, u.Character.LifeEpoch,
		"precondition: the death cascade is wired exactly once")

	assert.Equal(t, "bleeding out", cause,
		"the death cause must still read the held Bleeding record")

	assert.False(t, u.Character.HasCondition(conditions.ConditionIdBleeding), "the stripped bleed is gone after the respawn")
	assert.False(t, u.Character.HasCondition(conditions.ConditionIdConvictionWard), "the stripped shield is gone after the respawn")

	holderLines := drainPlain(1)
	roomLines := drainPlain(2)
	PruneConditions(events.NewTurn{TurnNumber: 1})
	holderLines = append(holderLines, drainPlain(1)...)
	roomLines = append(roomLines, drainPlain(2)...)

	assert.Equal(t, 0, countContaining(holderLines, "Your wounds stop bleeding."),
		"the respawned player must not read the stripped bleed's end line")
	assert.Equal(t, 0, countContaining(holderLines, "Conviction Ward fades"),
		"nor the stripped shield's")
	assert.Equal(t, 0, countContaining(roomLines, "ward around"),
		"and the room must not see it either")
}

// Control: a record that runs out on its own still narrates its end.
func TestDeathStrip_NaturalExpiryStillNarrates(t *testing.T) {
	u := setupConditionAfterDeath(t)
	t.Cleanup(conditions.SeedConditionRecordsForTest())

	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 4, -1, "claws"))
	expire(t, u.Character.Conditions.List, conditions.ConditionIdBleeding)
	drainPlain(1)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(1), "Your wounds stop bleeding."))
}

// Control: a strip that is not a death (the same All cancel, on a living
// player) still narrates at the prune. Only the death cascade is silent.
func TestDeathStrip_ANonDeathCancelStillNarrates(t *testing.T) {
	u := setupConditionAfterDeath(t)
	t.Cleanup(conditions.SeedConditionRecordsForTest())

	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 10, 3, "spell"))
	u.Character.CancelConditionsWithFlag(conditions.All)
	drainPlain(1)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(1), "Your Conviction Ward fades."))
}
