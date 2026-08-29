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
