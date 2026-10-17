package combatphase

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// U12c-2 Task 2. RoundsWaiting moves off the Aggro struct and onto this
// machine. It is a MACHINE field rather than per-state data because the ~20
// special-move `= 1` writes set it while Engaged, where EngagingData does not
// exist. See the two-counter note above the Machine struct.

func TestRoundsWaiting_SurvivesEngagingToEngaged(t *testing.T) {
	m := NewMachine()
	target := state.ActorRef{MobInstanceId: 100}

	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: target, RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand}))

	m.SetRoundsWaiting(3)
	assert.Equal(t, 3, m.RoundsWaiting())

	m.OnRoundTick() // RoundsUntil 1 -> 0, advances to Engaged
	require.Equal(t, Engaged, m.State())

	assert.Equal(t, 3, m.RoundsWaiting(),
		"RoundsWaiting is the ACTOR's round budget and must outlive the "+
			"wind-up; the ~20 special-move writes set it while Engaged")
}

func TestRoundsWaiting_ClearedOnIdle(t *testing.T) {
	m := NewMachine()
	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: state.ActorRef{MobInstanceId: 100}, RoundsUntil: 0},
		state.TransitionReason{Trigger: TriggerAttackCommand}))
	m.SetRoundsWaiting(5)

	m.ForceIdle(state.TransitionReason{Trigger: TriggerForceIdle})

	assert.Equal(t, Idle, m.State())
	assert.Zero(t, m.RoundsWaiting(),
		"EndAggro used to nil the whole Aggro struct, so the counter died "+
			"with the engagement; Idle must preserve that exactly")
}

func TestRoundsWaiting_DecrementStopsAtZero(t *testing.T) {
	m := NewMachine()
	m.SetRoundsWaiting(1)
	assert.True(t, m.ConsumeRoundWaiting(), "1 -> 0 consumes the round")
	assert.Zero(t, m.RoundsWaiting())
	assert.False(t, m.ConsumeRoundWaiting(), "already zero: nothing to consume")
	assert.Zero(t, m.RoundsWaiting(), "and it must not go negative")
}

// SpendRoundOnAction is a one-round budget whose wait round consumes
// silently. The mark lasts exactly as long as that round: consuming it, or
// any later SetRoundsWaiting (an engagement wind-up), clears it.
func TestRoundsWaiting_SpendRoundOnActionMarksOnlyThatRound(t *testing.T) {
	m := NewMachine()
	m.SpendRoundOnAction()
	assert.Equal(t, 1, m.RoundsWaiting())
	assert.True(t, m.RoundSpentByAction())
	assert.True(t, m.ConsumeRoundWaiting())
	assert.False(t, m.RoundSpentByAction(), "the spent round is gone, so is the mark")

	m.SpendRoundOnAction()
	m.SetRoundsWaiting(2)
	assert.False(t, m.RoundSpentByAction(), "an ordinary wait replaces the spent round and narrates")
}

func TestRoundsWaiting_NegativeSeedClampsToZero(t *testing.T) {
	m := NewMachine()
	m.SetRoundsWaiting(-4)
	assert.Zero(t, m.RoundsWaiting(),
		"every caller means 'wait at least this long', never 'act early'")
}
