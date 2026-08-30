package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Engagement-accessor contracts that survive the collapse.
//
// This file began as U12c-1's equivalence gate, proving IsInCombat() and
// CurrentCombatTarget() agreed with the raw .Aggro reads they were about to
// replace at ~241 sites. The field is gone, so most of that table became a
// tautology comparing the accessors to themselves; it is deleted rather than
// left to look like coverage.
//
// What remains are the two contracts that were never about the field.

// A refused commit must change nothing. This is the case U12c-0b fixed: before
// it, a vetoed transition still wrote the target, leaving the two stores
// disagreeing while BOTH were non-zero.
func TestEngagement_ARefusedCommitChangesNothing(t *testing.T) {
	c := New()
	c.SetAggro(0, 100, DefaultAttack)
	c.CombatPhase.RegisterTargetLifeCheck(func(state.ActorRef) bool { return false })

	c.SetAggro(0, 200, DefaultAttack)

	require.True(t, c.IsInCombat(), "the original engagement survives")
	assert.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId,
		"the refused commit left the previous target in place")
}

// U12c-2 landed: SetCast now records the cast on the Activity machine and no
// longer assigns Aggro, so the disagreement this used to pin is GONE.
//
// It was pinned as TestAccessors_KnownDisagreement_SetCastOverALiveEngagement,
// whose comment said: "If this test starts FAILING, that work happened and the
// assertion below should be inverted into an equivalence assertion, not
// deleted." This is that inversion.
//
// What used to happen: SetCast assigned c.Aggro directly and never touched
// CombatPhase, so calling it over a live engagement dropped Aggro to zero ids
// while CombatPhase kept the old target, and CurrentCombatTarget() reported the
// stale one.
func TestAccessors_AgreeAfterSetCastOverALiveEngagement(t *testing.T) {
	c := New()
	c.SetAggro(0, 100, DefaultAttack)
	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId)

	require.True(t, c.SetCast(2, SpellAggroInfo{SpellId: "aidskill", TargetUserIds: []int{7}}),
		"precondition: the cast was recorded")

	assert.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId,
		"the engagement is untouched by starting a cast; both stores agree")
	assert.True(t, c.IsInCombat())
	assert.True(t, c.IsCasting(), "and the cast is recorded on the Activity machine")

	// The aim lives in CastingData, never in the combat target.
	cd, ok := c.CastingData()
	require.True(t, ok)
	assert.Equal(t, []int{7}, cd.TargetUserIds)
	assert.True(t, c.IsAggro(7, 0), "IsAggro still sees the spell target")
}

// A cast from idle has nothing to go stale, so the accessors agree. Pinned as
// the boundary of the exception above: the disagreement needs a PRIOR
// engagement, it is not inherent to SetCast.
func TestAccessors_AgreeWhenSetCastComesFromIdle(t *testing.T) {
	c := New()
	require.True(t, c.SetCast(2, SpellAggroInfo{SpellId: "aidskill", TargetUserIds: []int{7}}))

	assert.True(t, c.IsCasting(), "the cast is recorded")
	assert.Equal(t, state.ActorRef{}, c.CurrentCombatTarget(),
		"a cast from idle sets no combat target")

	// ⚠️ U12c-2 BEHAVIOUR CHANGE, deliberate: a pending cast no longer counts
	// as "in combat". It used to, only because SetCast assigned Aggro and
	// IsInCombat fell back to `Aggro != nil`. Casting is an Activity, not a
	// combat phase, and conflating them is what let a cast look like an
	// engagement with no target -- the stale state ValidateAggro had a special
	// exemption for.
	assert.False(t, c.IsInCombat(), "casting is an activity, not an engagement")

	// The aim lives in CastingData, which only IsAggro and
	// targeting.EngagementOf consult -- NOT CurrentCombatTarget.
	assert.True(t, c.IsAggro(7, 0), "IsAggro still sees the spell target")
}
