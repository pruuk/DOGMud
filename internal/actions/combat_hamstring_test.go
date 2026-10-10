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

// TestHamstring_BleedNamesAttacker pins that a landed hamstring stamps its
// attacker on the Bleeding record, so a bleed kill credits them (#240).
// Retries until a hit lands (extreme Dexterity vs 1) rather than mocking the
// dice.
func TestHamstring_BleedNamesAttacker(t *testing.T) {
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		7313: {SpeciesId: 7313, Name: "wolf", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
	})
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	targetMob := &mobs.Mob{InstanceId: 7393}
	targetMob.Character.Name = "Target"
	targetMob.Character.HealthMax.Value = 100000
	targetMob.Character.Health = 100000
	targetMob.Character.Stats.Dexterity.ValueAdj = 1
	targetMob.Character.Conditions = conditions.New()
	setCombatPositionParallel(&targetMob.Character, position.Standing)
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

	char := characters.New()
	char.SpeciesId = 7313
	char.Stats.Strength.ValueAdj = 500
	char.Stats.Dexterity.ValueAdj = 500
	attacker := &idStubActor{stubActor: newStubActor(char, newTestRoom()), mobInstanceId: 4346}

	hitSeen := false
	for i := 0; i < 100 && !hitSeen; i++ {
		fundSpecialMove(char)
		char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
		char.Cooldowns = characters.Cooldowns{}
		res := ExecuteHamstring(attacker)
		hitSeen = res.Executed && res.MoveResult.Hit
	}
	if !hitSeen {
		t.Fatal("no hamstring hit in 100 attempts; the hit path is broken")
	}

	held := targetMob.Character.GetConditions(conditions.ConditionIdBleeding)
	if assert.NotEmpty(t, held, "a landed hamstring leaves a Bleeding record") {
		want := ActorRefOf(attacker)
		assert.False(t, want.IsZero(), "the test attacker must have a real identity")
		assert.Equal(t, want, held[0].Caster, "the hamstring bleed names its attacker (#240)")
	}
}
