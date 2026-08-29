package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommit_SetsTheTarget(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)

	require.NotNil(t, c.Aggro)
	assert.Equal(t, 42, c.Aggro.MobInstanceId)
	assert.Equal(t, 42, EngagementOf(c).Target.MobInstanceId)
}

func TestCommit_SurpriseReasonArmsTheOpening(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonSurprise)

	assert.True(t, EngagementOf(c).OpeningUnspent)
}

// TestCommit_DualWriteAgrees pins the invariant that SetAggro maintains by
// convention today: after any commit, the two stores describe the same
// engagement. U12c deletes one of them; until then this is what stops them
// drifting.
func TestCommit_DualWriteAgrees(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)

	require.NotNil(t, c.Aggro)
	require.NotNil(t, c.CombatPhase)
	assert.True(t, c.CombatPhase.IsInCombat(),
		"CombatPhase must agree that a commit started a fight")
}

func TestRelease_ClearsTheTarget(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)

	Release(c, ReasonDisengage)

	assert.Nil(t, c.Aggro)
	assert.True(t, EngagementOf(c).Target.IsZero())
}

func TestCommitAndRelease_NilCharacterDoNotPanic(t *testing.T) {
	assert.NotPanics(t, func() { Commit(nil, state.ActorRef{MobInstanceId: 1}, ReasonAttack) })
	assert.NotPanics(t, func() { Release(nil, ReasonDisengage) })
}

// TestCommit_ZeroRefIsRefused: a zero ActorRef means "nobody". Committing to
// nobody would set an engagement with no target, which every downstream
// consumer then has to defend against.
func TestCommit_ZeroRefIsRefused(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{}, ReasonAttack)

	assert.Nil(t, c.Aggro)
}

// TestCommitAfter_PassesTheExplicitWait proves CommitAfter is not just Commit
// with an ignored argument. SetAggro sums its variadic when present and falls
// back to weapon speed when absent, so an explicit 3 must survive.
func TestCommitAfter_PassesTheExplicitWait(t *testing.T) {
	c := characters.New()

	CommitAfter(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack, 3)

	require.NotNil(t, c.Aggro)
	assert.Equal(t, 3, c.Aggro.RoundsWaiting)
}

func TestCommitTaunt_PinsTheTargetOntoTheTaunter(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 50}, ReasonAttack)

	CommitTaunt(c, state.ActorRef{UserId: 7}, 4)

	assert.Equal(t, 7, EngagementOf(c).Target.UserId)
	assert.Equal(t, 0, EngagementOf(c).Target.MobInstanceId)
}

// TestCommitTaunt_HoldSurvivesReaggro is the whole point of the mechanic: an
// ally swinging at the taunted mob must not flip it back off the taunter.
func TestCommitTaunt_HoldSurvivesReaggro(t *testing.T) {
	c := characters.New()
	CommitTaunt(c, state.ActorRef{UserId: 7}, 4)

	Commit(c, state.ActorRef{MobInstanceId: 50}, ReasonAttack)

	assert.Equal(t, 7, EngagementOf(c).Target.UserId,
		"a basic re-aggro must not break an active taunt hold")
}

// The hold is set BEFORE the commit so the gate sees the new taunter as the
// locked target and lets this very set through. If the two lines are reversed,
// a taunt cannot override an existing hold and silently no-ops.
func TestCommitTaunt_NewerTauntOverridesActiveHold(t *testing.T) {
	c := characters.New()
	CommitTaunt(c, state.ActorRef{MobInstanceId: 50}, 4)

	CommitTaunt(c, state.ActorRef{MobInstanceId: 60}, 4)

	assert.Equal(t, 60, EngagementOf(c).Target.MobInstanceId)
}

func TestCommitTaunt_NilAndZeroAreSafe(t *testing.T) {
	assert.NotPanics(t, func() { CommitTaunt(nil, state.ActorRef{UserId: 7}, 4) })

	c := characters.New()
	CommitTaunt(c, state.ActorRef{}, 4)
	assert.Nil(t, c.Aggro, "a taunt with no taunter must not engage anybody")
}
