package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// ExecuteMaul tests
// ---------------------------------------------------------------------------

// TestMaul_NoAggro verifies that ExecuteMaul returns NoTarget=true when the
// actor has no aggro set (not yet in combat).
func TestMaul_NoAggro(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	result := ExecuteMaul(actor)

	assert.False(t, result.Executed, "maul with no aggro should not execute")
	assert.True(t, result.NoTarget, "maul with no aggro should set NoTarget")
	assert.False(t, result.OnCooldown, "NoTarget should take priority over cooldown reporting")
}

// TestMaul_OnCooldown verifies that ExecuteMaul returns OnCooldown=true when
// the special-move cooldown is active.
func TestMaul_OnCooldown(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	prepareSpecialMoveCooldown(t, char, 7904, 7904, &species.Species{
		SpeciesId: 7904, Name: "cooldown-wolf", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite,
	})

	result := ExecuteMaul(actor)

	assert.False(t, result.Executed, "maul should not execute when on cooldown")
	assert.True(t, result.OnCooldown, "maul should report OnCooldown")
}

// TestMaul_NotFanged verifies the anatomy/identity gate: a non-fanged actor in
// combat with a valid target gets NotFanged=true and Executed=false, while a
// fanged actor passes the gate and executes the move.
func TestMaul_NotFanged(t *testing.T) {
	// Seed two species: one clawed (not fanged), one fanged.
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		5001: {SpeciesId: 5001, Name: "feline", BodyParts: []string{"legs", "paws"}, NaturalAttack: items.Claws},
		5002: {SpeciesId: 5002, Name: "wolf", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
	})
	defer cleanup()

	// Register a target mob so ResolveAggroTarget returns Found=true.
	targetMob := &mobs.Mob{InstanceId: 5099}
	targetMob.Character.Name = "Target"
	targetMob.Character.HealthMax.Value = 100
	targetMob.Character.Health = 100
	setCombatPositionParallel(&targetMob.Character, position.Standing)
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

	t.Run("non-fanged actor returns NotFanged", func(t *testing.T) {
		char := characters.New()
		char.SpeciesId = 5001 // feline — clawed, not fanged
		char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)

		result := ExecuteMaul(newStubActor(char, newTestRoom()))

		assert.True(t, result.NotFanged, "non-fanged actor should return NotFanged=true")
		assert.False(t, result.Executed, "non-fanged actor should not execute the maul")
		assert.Equal(t, 0, result.MoveResult.Damage, "non-fanged maul should deal no damage")
	})

	t.Run("fanged actor passes the gate and executes", func(t *testing.T) {
		char := characters.New()
		char.SpeciesId = 5002 // wolf — fanged
		char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
		fundSpecialMove(char)

		result := ExecuteMaul(newStubActor(char, newTestRoom()))

		assert.False(t, result.NotFanged, "fanged actor should NOT return NotFanged")
		assert.True(t, result.Executed, "fanged actor should execute the maul")
	})
}

// TestMaul_TargetGone verifies that when aggro is set to an invalid mob
// instance ID (target gone), Executed is false and NoTarget is true.
func TestMaul_TargetGone(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	// Aggro pointing at a nonexistent mob instance — target resolution fails.
	char.SetAggro(0, 999999, characters.DefaultAttack)

	result := ExecuteMaul(actor)

	// Target validation precedes cooldown admission, so OnCooldown is false.
	// Target resolution then fails → NoTarget.
	assert.False(t, result.Executed, "maul with missing target should not execute")
	assert.True(t, result.NoTarget, "maul should report NoTarget when the resolved target is gone")
	assert.False(t, result.OnCooldown, "cooldown should not be reported when target is gone")
}

// TestMaul_BleedNamesAttacker pins that a landed maul stamps its attacker on
// the Bleeding record, so a bleed kill credits them (#240). Retries until a
// hit lands (extreme Dexterity vs 1) rather than mocking the dice.
func TestMaul_BleedNamesAttacker(t *testing.T) {
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		7312: {SpeciesId: 7312, Name: "wolf", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
	})
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	targetMob := &mobs.Mob{InstanceId: 7392}
	targetMob.Character.Name = "Target"
	targetMob.Character.HealthMax.Value = 100000
	targetMob.Character.Health = 100000
	targetMob.Character.Stats.Dexterity.ValueAdj = 1
	targetMob.Character.Conditions = conditions.New()
	setCombatPositionParallel(&targetMob.Character, position.Standing)
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

	char := characters.New()
	char.SpeciesId = 7312
	char.Stats.Strength.ValueAdj = 500
	char.Stats.Dexterity.ValueAdj = 500
	attacker := &idStubActor{stubActor: newStubActor(char, newTestRoom()), mobInstanceId: 4344}

	hitSeen := false
	for i := 0; i < 100 && !hitSeen; i++ {
		fundSpecialMove(char)
		char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
		char.Cooldowns = characters.Cooldowns{}
		res := ExecuteMaul(attacker)
		hitSeen = res.Executed && res.MoveResult.Hit
	}
	if !hitSeen {
		t.Fatal("no maul hit in 100 attempts; the hit path is broken")
	}

	held := targetMob.Character.GetConditions(conditions.ConditionIdBleeding)
	if assert.NotEmpty(t, held, "a landed maul leaves a Bleeding record") {
		want := ActorRefOf(attacker)
		assert.False(t, want.IsZero(), "the test attacker must have a real identity")
		assert.Equal(t, want, held[0].Caster, "the maul bleed names its attacker (#240)")
	}
}
