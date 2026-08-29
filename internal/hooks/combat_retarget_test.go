package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/stretchr/testify/assert"
)

// ValidateAggro must treat Flee-type aggro (with no UserId / MobInstanceId)
// as valid — a fleeing player's Aggro intentionally has no target.
// Without this, the next combat round's ValidateAggro returns false, the
// caller runs RetargetOrEnd which acquires a new target and clears the Flee
// type, and the player never actually flees. Regression for the flee bug
// reported 2026-04-22.
// U12c-2: the Flee AggroType is dissolved into the Disengaging combat phase,
// so this drives the real transition rather than constructing a sentinel.
//
// 📌 FINDING, recorded rather than hidden: the "no target" half of the old test
// is UNREACHABLE, and always was.
//
//   - TransitionToDisengaging seeds DisengagingData.LastTarget from
//     CurrentTarget(), and only Engaged (which always has a target) may
//     transition to Disengaging. So a disengaging actor's ref is never zero,
//     and ValidateAggro's zero-ref branch cannot fire for one.
//   - The Flee exemption it replaced was equally dead: nothing in production
//     ever set Aggro.Type = Flee. flee.go always went through
//     TransitionToDisengaging.
//
// The exemption is kept as belt-and-braces (it costs one call and defends the
// contract if LastTarget ever becomes optional), but it is not what this test
// exercises. What IS reachable, and what this pins, is that a disengaging
// actor with a live present target keeps its engagement.
func TestValidateAggro_DisengagingWithLiveTarget_IsValid(t *testing.T) {
	target := &mobs.Mob{
		MobId:      7,
		InstanceId: 700,
		Character:  *characters.New(),
	}
	target.Character.Name = "Quarry"
	target.Character.Health = 50
	target.Character.RoomId = 1
	cleanup := mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{700: target})
	defer cleanup()

	c := characters.New()
	c.RoomId = 1
	c.SetAggro(0, 700, characters.DefaultAttack)
	c.CombatPhase.OnRoundTick() // Engaging -> Engaged; only Engaged may disengage
	require.Equal(t, combatphase.Engaged, c.CombatPhase.State())

	require.NoError(t, c.CombatPhase.TransitionToDisengaging(state.TransitionReason{
		Trigger: combatphase.TriggerFleeCommand,
	}))
	require.True(t, c.IsDisengaging(), "precondition: the actor is disengaging")
	require.Equal(t, 700, c.CurrentCombatTarget().MobInstanceId,
		"a disengagement carries the target it is fleeing FROM")

	ok := ValidateAggro(c)
	assert.True(t, ok, "a disengaging actor with a live target stays valid")
	assert.True(t, c.IsDisengaging(), "ValidateAggro must not end a disengagement")
}

// Parity check: SpellCast-type aggro with no target is already treated as
// valid (existing behavior). Locking it in.
func TestValidateAggro_SpellCastTypeWithNoTarget_IsValid(t *testing.T) {
	c := characters.New()
	c.Aggro = &characters.Aggro{
		UserId:        0,
		MobInstanceId: 0,
		Type:          characters.SpellCast,
	}

	ok := ValidateAggro(c)
	assert.True(t, ok, "SpellCast aggro with no target should be valid")
	assert.NotNil(t, c.Aggro, "ValidateAggro must not end a SpellCast aggro")
}

// DefaultAttack with no target is stale state — ValidateAggro should
// invalidate it.
func TestValidateAggro_DefaultAttackWithNoTarget_IsInvalid(t *testing.T) {
	c := characters.New()
	c.Aggro = &characters.Aggro{
		UserId:        0,
		MobInstanceId: 0,
		Type:          characters.DefaultAttack,
	}

	ok := ValidateAggro(c)
	assert.False(t, ok, "DefaultAttack with no target is stale, should be invalid")
	assert.Nil(t, c.Aggro, "EndAggro must have cleared aggro")
}

// Nil aggro returns false without side effects.
func TestValidateAggro_NilAggro(t *testing.T) {
	c := characters.New()
	c.Aggro = nil

	ok := ValidateAggro(c)
	assert.False(t, ok)
	assert.Nil(t, c.Aggro)
}
