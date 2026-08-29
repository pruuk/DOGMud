package combatphase

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/require"
)

func retargetTestActor(id int) state.ActorRef {
	return state.ActorRef{MobInstanceId: id}
}

// engageFully drives a machine from Idle to Engaged against target, the way
// the round driver does: transition, then tick until the wind-up expires.
func engageFully(t *testing.T, m *Machine, target state.ActorRef, roundsUntil int) {
	t.Helper()
	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: target, RoundsUntil: roundsUntil},
		state.TransitionReason{Trigger: TriggerAttackCommand}))
	for i := 0; i < roundsUntil+2 && m.State() != Engaged; i++ {
		m.OnRoundTick()
	}
	require.Equal(t, Engaged, m.State(), "fixture must reach Engaged")
}

// TestRetarget_WhileEngagedUpdatesTheTarget is the U12c-0 regression.
//
// Before this slice, validTransitions declared Engaged: {Disengaging, Idle},
// so TransitionToEngaging returned ErrInvalidTransition on a retarget and
// SetAggro discarded it. CurrentTarget kept returning the PREVIOUS enemy,
// which is what the {target} and {targethealth} prompt tokens render.
func TestRetarget_WhileEngagedUpdatesTheTarget(t *testing.T) {
	m := NewMachine()
	engageFully(t, m, retargetTestActor(100), 1)
	require.Equal(t, retargetTestActor(100), m.CurrentTarget())

	err := m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand})

	require.NoError(t, err, "a retarget while Engaged must be a legal transition")
	require.Equal(t, retargetTestActor(200), m.CurrentTarget(),
		"CurrentTarget must follow the retarget, not keep the old enemy")
}

// A retarget re-imposes the wind-up. That is the intended behaviour change:
// switching targets mid-fight takes a moment, and SetAggro already reseeds
// RoundsWaiting on every retarget, so the moment was already being paid.
func TestRetarget_ReimposesTheWindUp(t *testing.T) {
	m := NewMachine()
	engageFully(t, m, retargetTestActor(100), 1)

	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 2},
		state.TransitionReason{Trigger: TriggerAttackCommand}))

	require.Equal(t, Engaging, m.State(),
		"a retarget re-enters Engaging rather than staying Engaged")

	d, ok := m.EngagingData()
	require.True(t, ok)
	require.Equal(t, 2, d.RoundsUntil, "the new wind-up is the one supplied")

	m.OnRoundTick()
	m.OnRoundTick()
	require.Equal(t, Engaged, m.State(), "and it advances back to Engaged")
	require.Equal(t, retargetTestActor(200), m.CurrentTarget())
}

// The superseded Engaged data must not survive the retarget. The public
// accessors are state-gated so a stale value is invisible today, but leaving
// it sets a trap for any future accessor that is not.
func TestRetarget_ClearsTheSupersededEngagedData(t *testing.T) {
	m := NewMachine()
	engageFully(t, m, retargetTestActor(100), 1)

	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand}))

	require.Nil(t, m.engaged,
		"the superseded Engaged data must be cleared, not left behind")
}

// A retarget still runs the target vetoes. This is why the fix allows the
// transition rather than mutating m.engaged in place: an in-place setter would
// skip the vetoes and could leave the machine pointing at a corpse.
func TestRetarget_StillHonoursTargetVetoes(t *testing.T) {
	m := NewMachine()
	engageFully(t, m, retargetTestActor(100), 1)

	m.RegisterTargetLifeCheck(func(state.ActorRef) bool { return false })

	err := m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand})

	require.Error(t, err, "a retarget onto a dead target must be refused")
	require.Equal(t, retargetTestActor(100), m.CurrentTarget(),
		"a refused retarget leaves the existing engagement intact")
}
