package combatphase

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A one-day-old regression, found by the U12c-2 adversarial playtest.
//
// U12c-0 and U12c-0b made a RETARGET land, which was the right fix for a stale
// {target} prompt. But a retarget goes Engaged -> Engaging, and the original
// 2026-05-13 table had no Engaging -> Disengaging edge. That was harmless while
// nothing could park an actor in Engaging; once retargets landed, anything that
// retargets you every round holds you there, and `flee` is refused EVERY round.
//
// Observed in play as ~15 consecutive refusals of "You can't break away just
// yet." while standing and un-grappled.

func engagedActor(t *testing.T) *Machine {
	t.Helper()
	m := NewMachine()
	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: state.ActorRef{MobInstanceId: 700}, RoundsUntil: 0},
		state.TransitionReason{Trigger: TriggerAttackCommand}))
	m.OnRoundTick()
	require.Equal(t, Engaged, m.State(), "precondition: the wind-up finished")
	return m
}

func fleeFrom(m *Machine) error {
	return m.TransitionToDisengaging(state.TransitionReason{Trigger: TriggerFleeCommand})
}

// The direct case: a retarget puts you in the wind-up, and you must still be
// able to run.
func TestFlee_IsAllowedDuringTheWindUp(t *testing.T) {
	m := engagedActor(t)

	// A retarget, exactly as reciprocal aggro or the `target` command does it.
	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: state.ActorRef{MobInstanceId: 800}, RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand}))
	require.Equal(t, Engaging, m.State(), "precondition: a retarget re-enters the wind-up")

	assert.NoError(t, fleeFrom(m), "an actor mid wind-up must still be able to flee")
	assert.Equal(t, Disengaging, m.State())
}

// The case that actually bit: retargeted every round, so the wind-up never
// ends. Without the Engaging -> Disengaging edge this fails on every iteration
// and the actor can never escape.
func TestFlee_SurvivesBeingRetargetedEveryRound(t *testing.T) {
	m := engagedActor(t)

	for i := 0; i < 5; i++ {
		m.OnRoundTick()
		require.NoError(t, m.TransitionToEngaging(
			EngagingData{Target: state.ActorRef{MobInstanceId: 800 + i}, RoundsUntil: 1},
			state.TransitionReason{Trigger: TriggerAttackCommand}),
			"round %d: the retarget itself must keep working", i)
	}
	require.Equal(t, Engaging, m.State(), "precondition: still parked in the wind-up")

	assert.NoError(t, fleeFrom(m),
		"being retargeted every round must not trap an actor in a fight forever")
	assert.Equal(t, Disengaging, m.State())
}

// A failed flee returns you to Engaged, which the Disengaging row already
// allows. Pinned because fleeing from Engaging means you can now land in
// Engaged without having finished the wind-up you started.
func TestFlee_FailureFromTheWindUpReturnsToEngaged(t *testing.T) {
	m := engagedActor(t)
	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: state.ActorRef{MobInstanceId: 800}, RoundsUntil: 2},
		state.TransitionReason{Trigger: TriggerAttackCommand}))
	require.NoError(t, fleeFrom(m))

	require.NoError(t, m.Inner().TransitionTo(Engaged,
		state.TransitionReason{Trigger: TriggerEngagementReady}),
		"a failed flee must be able to put the actor back into the fight")
	assert.Equal(t, Engaged, m.State())
}
