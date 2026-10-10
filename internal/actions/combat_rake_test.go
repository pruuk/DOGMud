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
// ExecuteRake tests
// ---------------------------------------------------------------------------

// TestRake_NoAggro verifies that ExecuteRake returns NoTarget=true when the
// actor has no aggro set (not yet in combat).
func TestRake_NoAggro(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	result := ExecuteRake(actor)

	assert.False(t, result.Executed, "rake with no aggro should not execute")
	assert.True(t, result.NoTarget, "rake with no aggro should set NoTarget")
	assert.False(t, result.OnCooldown, "NoTarget should take priority over cooldown reporting")
}

// TestRake_OnCooldown verifies that ExecuteRake returns OnCooldown=true when
// the special-move cooldown is active.
func TestRake_OnCooldown(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	prepareSpecialMoveCooldown(t, char, 7903, 7903, &species.Species{
		SpeciesId: 7903, Name: "cooldown-feline", BodyParts: []string{"legs"}, NaturalAttack: items.Claws,
	})

	result := ExecuteRake(actor)

	assert.False(t, result.Executed, "rake should not execute when on cooldown")
	assert.True(t, result.OnCooldown, "rake should report OnCooldown")
}

// TestRake_NotClawed verifies the anatomy/identity gate: a non-clawed actor in
// combat with a valid target gets NotClawed=true and Executed=false, while a
// clawed actor passes the gate and executes the move.
func TestRake_NotClawed(t *testing.T) {
	// Seed two species: one fanged (not clawed), one clawed.
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		5001: {SpeciesId: 5001, Name: "wolf", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
		5002: {SpeciesId: 5002, Name: "feline", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Claws},
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

	t.Run("non-clawed actor returns NotClawed", func(t *testing.T) {
		char := characters.New()
		char.SpeciesId = 5001 // wolf — fanged, not clawed
		char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)

		result := ExecuteRake(newStubActor(char, newTestRoom()))

		assert.True(t, result.NotClawed, "non-clawed actor should return NotClawed=true")
		assert.False(t, result.Executed, "non-clawed actor should not execute the rake")
		assert.Equal(t, 0, result.MoveResult.Damage, "non-clawed rake should deal no damage")
	})

	t.Run("clawed actor passes the gate and executes", func(t *testing.T) {
		char := characters.New()
		char.SpeciesId = 5002 // feline — clawed
		char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
		fundSpecialMove(char)

		result := ExecuteRake(newStubActor(char, newTestRoom()))

		assert.False(t, result.NotClawed, "clawed actor should NOT return NotClawed")
		assert.True(t, result.Executed, "clawed actor should execute the rake")
	})
}

// TestRake_TargetGone verifies that when aggro is set to an
// invalid mob instance ID (target gone), Executed is false and NoTarget
// is true without consuming the cooldown.
func TestRake_TargetGone(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	// Aggro pointing at a nonexistent mob instance — target resolution fails.
	char.SetAggro(0, 999999, characters.DefaultAttack)

	result := ExecuteRake(actor)

	// Target validation precedes cooldown admission, so OnCooldown is false.
	// Target resolution then fails → NoTarget.
	assert.False(t, result.Executed, "rake with missing target should not execute")
	assert.True(t, result.NoTarget, "rake should report NoTarget when the resolved target is gone")
	assert.False(t, result.OnCooldown, "cooldown should not be reported when target is gone")
}

// TestRake_BleedNamesAttacker pins that a landed rake stamps its attacker on
// the Bleeding record, so a bleed kill credits them (#240). Retries until a
// hit lands (extreme Dexterity vs 1) rather than mocking the dice.
func TestRake_BleedNamesAttacker(t *testing.T) {
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		7311: {SpeciesId: 7311, Name: "feline", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Claws},
	})
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	targetMob := &mobs.Mob{InstanceId: 7391}
	targetMob.Character.Name = "Target"
	targetMob.Character.HealthMax.Value = 100000
	targetMob.Character.Health = 100000
	targetMob.Character.Stats.Dexterity.ValueAdj = 1
	targetMob.Character.Conditions = conditions.New()
	setCombatPositionParallel(&targetMob.Character, position.Standing)
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

	char := characters.New()
	char.SpeciesId = 7311
	char.Stats.Strength.ValueAdj = 500
	char.Stats.Dexterity.ValueAdj = 500
	attacker := &idStubActor{stubActor: newStubActor(char, newTestRoom()), mobInstanceId: 4343}

	hitSeen := false
	for i := 0; i < 100 && !hitSeen; i++ {
		fundSpecialMove(char)
		char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
		char.Cooldowns = characters.Cooldowns{}
		res := ExecuteRake(attacker)
		hitSeen = res.Executed && res.MoveResult.Hit
	}
	if !hitSeen {
		t.Fatal("no rake hit in 100 attempts; the hit path is broken")
	}

	held := targetMob.Character.GetConditions(conditions.ConditionIdBleeding)
	if assert.NotEmpty(t, held, "a landed rake leaves a Bleeding record") {
		want := ActorRefOf(attacker)
		assert.False(t, want.IsZero(), "the test attacker must have a real identity")
		assert.Equal(t, want, held[0].Caster, "the rake bleed names its attacker (#240)")
	}
}
