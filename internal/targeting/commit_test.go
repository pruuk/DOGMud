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

	require.True(t, c.IsInCombat())
	assert.Equal(t, 42, c.CurrentCombatTarget().MobInstanceId)
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

	require.True(t, c.IsInCombat())
	require.NotNil(t, c.CombatPhase)
	assert.True(t, c.CombatPhase.IsInCombat(),
		"CombatPhase must agree that a commit started a fight")
}

func TestRelease_ClearsTheTarget(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)

	Release(c, ReasonDisengage)

	assert.False(t, c.IsInCombat())
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

	assert.False(t, c.IsInCombat())
}

// TestCommitAfter_PassesTheExplicitWait proves CommitAfter is not just Commit
// with an ignored argument. SetAggro sums its variadic when present and falls
// back to weapon speed when absent, so an explicit 3 must survive.
func TestCommitAfter_PassesTheExplicitWait(t *testing.T) {
	c := characters.New()

	CommitAfter(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack, 3)

	require.True(t, c.IsInCombat())
	assert.Equal(t, 3, c.RoundsWaiting())
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
	assert.False(t, c.IsInCombat(), "a taunt with no taunter must not engage anybody")
}

// TestReasonRoundTrip pins the transform a blind review caught being lossy.
//
// usercommands/target.go reads Aggro.Type and hands it straight back through
// ReasonForAggroType. Its own gate permits Shooting, so collapsing Shooting to
// DefaultAttack was identical ONLY while the weapon was still a Shooting
// subtype (SetAggro re-infers it); swap the weapon between the shot and the
// target switch and the engagement quietly changed type.
//
// Flee and SpellCast are deliberately NOT round-tripped: no production site
// commits them, they are written only by SetCast and by tests, and inventing a
// writer for them would be a behaviour change of its own.
func TestReasonRoundTrip(t *testing.T) {
	for _, at := range []characters.AggroType{
		characters.DefaultAttack,
		characters.SurpriseAttack,
		characters.Shooting,
	} {
		assert.Equal(t, at, aggroTypeFor(ReasonForAggroType(at)),
			"AggroType %v must survive the round trip through Reason", at)
	}
}

// TestCommit_ShootReasonPreservesShootingWithoutAWeapon is the concrete
// regression: with no Shooting-subtype weapon equipped there is nothing for
// SetAggro to re-infer from, so only a faithful Reason keeps the type.
// ⚠️ U12c-2 CHANGED WHAT THIS CAN ASSERT, and the change is worth knowing.
//
// This pinned that a Shooting engagement SURVIVED a re-commit with no bow in
// hand, because "shooting-ness" was stored on the engagement. It is derived
// from the equipped weapon now (Engagement.Ranged), so it cannot outlive the
// bow: unequip mid-fight and the engagement stops being ranged immediately.
//
// That is the intended direction -- stored and derived state cannot disagree
// if only one exists -- but it means an engagement can no longer be ranged
// without a ranged weapon. Its one consumer is ordinaryMeleeEngagement, which
// gates mob melee AI, so the effect is that a disarmed archer now runs melee
// AI rather than staying flagged as a shooter forever.
func TestCommit_ShootReasonEngagesAndRangedFollowsTheWeapon(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonShoot)

	require.True(t, c.IsInCombat())
	assert.False(t, EngagementOf(c).Ranged,
		"with no bow in hand the engagement is not ranged, however it was committed")
}

// TestCommit_ReportsWhetherItLanded pins the U12c-0b return value. It exists
// because a bare "assume it worked" cost a nil-pointer panic in
// hooks.RetargetOrEnd, and because anything that narrates a fight to a player
// must be able to tell whether one started.
func TestCommit_ReportsWhetherItLanded(t *testing.T) {
	c := characters.New()

	require.True(t, Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack),
		"an unvetoed commit must report success")

	// Veto every subsequent target, as a dead or despawning one would.
	c.CombatPhase.RegisterTargetLifeCheck(func(state.ActorRef) bool { return false })

	require.False(t, Commit(c, state.ActorRef{MobInstanceId: 99}, ReasonAttack),
		"a vetoed commit must report failure, not silence")
	assert.Equal(t, 42, c.CurrentCombatTarget().MobInstanceId,
		"and must leave the previous engagement intact")
}

func TestCommit_ReportsFalseForNilAndZeroRef(t *testing.T) {
	assert.False(t, Commit(nil, state.ActorRef{MobInstanceId: 1}, ReasonAttack))
	assert.False(t, Commit(characters.New(), state.ActorRef{}, ReasonAttack))
	assert.False(t, CommitAfter(nil, state.ActorRef{MobInstanceId: 1}, ReasonAttack, 1))
	assert.False(t, CommitTaunt(nil, state.ActorRef{UserId: 7}, 4))
}

func TestCommitAfterAndTaunt_ReportWhetherTheyLanded(t *testing.T) {
	c := characters.New()
	require.True(t, CommitAfter(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack, 1))

	c.CombatPhase.RegisterTargetLifeCheck(func(state.ActorRef) bool { return false })

	assert.False(t, CommitAfter(c, state.ActorRef{MobInstanceId: 99}, ReasonAttack, 1))
	assert.False(t, CommitTaunt(c, state.ActorRef{UserId: 7}, 4),
		"a refused taunt pulled nobody and must say so")
}
