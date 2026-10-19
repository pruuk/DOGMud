package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const smokeSpeciesForTest = 7444

func seedSmotherFixture(t *testing.T, targetID, targetDex int) *mobs.Mob {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		perception.ConditionIdBlinded:   {ConditionId: perception.ConditionIdBlinded, Name: "Blinded", TriggerCount: 3, RoundInterval: 1},
		conditions.ConditionIdSmothered: {ConditionId: conditions.ConditionIdSmothered, Name: "Smothered", TriggerCount: 3, RoundInterval: 1},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		smokeSpeciesForTest: {SpeciesId: smokeSpeciesForTest, Name: "smoke-test", BodyParts: []string{}, BeastMoves: []string{"smother"}},
		7445:                {SpeciesId: 7445, Name: "clawed-unlisted", BodyParts: []string{"legs"}},
	}))
	target := &mobs.Mob{InstanceId: targetID}
	target.Character.Name = "Target"
	target.Character.HealthMax.Value = 10_000
	target.Character.Health = 10_000
	target.Character.StaminaMax.Value = 10_000
	target.Character.Stamina = 10_000
	target.Character.Stats.Dexterity.ValueAdj = targetDex
	target.Character.Conditions = conditions.New()
	setCombatPositionParallel(&target.Character, position.Standing)
	mobs.SetInstanceForTest(targetID, target)
	t.Cleanup(func() { mobs.SetInstanceForTest(targetID, nil) })
	return target
}

func newSmoke(targetID int) *characters.Character {
	char := characters.New()
	char.SpeciesId = smokeSpeciesForTest
	char.Stats.Dexterity.ValueAdj = 500
	char.SetAggro(0, targetID, characters.DefaultAttack)
	fundSpecialMove(char)
	return char
}

func TestSmother_UnlistedSpeciesIsRefused(t *testing.T) {
	seedSmotherFixture(t, 7460, 1)
	char := newSmoke(7460)
	char.SpeciesId = 7445
	res := ExecuteSmother(newStubActor(char, newTestRoom()))
	assert.True(t, res.NotSmoke)
	assert.False(t, res.Executed)
}

// Appendix A: a hit blinds and smothers; anything else applies neither.
func TestSmother_HitBlindsAndSmothers_OtherwiseNothing(t *testing.T) {
	sawHit, sawOther := false, false
	for _, dex := range []int{1, 5_000} {
		target := seedSmotherFixture(t, 7461, dex)
		for i := 0; i < 100; i++ {
			target.Character.Conditions = conditions.New()
			char := newSmoke(7461)
			res := ExecuteSmother(newStubActor(char, newTestRoom()))
			require.True(t, res.Executed)
			blind := target.Character.Conditions.TriggersLeft(perception.ConditionIdBlinded) > 0
			choking := target.Character.Conditions.TriggersLeft(conditions.ConditionIdSmothered) > 0
			if res.MoveResult.Hit {
				sawHit = true
				assert.True(t, blind && choking, "a hit must blind and smother")
			} else {
				sawOther = true
				assert.False(t, blind || choking, "a partial or miss must apply neither")
			}
		}
	}
	require.True(t, sawHit, "no hit in 200 tries: the fixture is broken")
	require.True(t, sawOther, "no partial or miss in 200 tries: the fixture is broken")
}

// Appendix A: the cast interrupt rides the shared special-move cooldown, so
// a second smother inside the window is refused before it can choke again.
func TestSmother_SecondSmotherInsideTheCooldownIsRefused(t *testing.T) {
	seedSmotherFixture(t, 7462, 1)
	char := newSmoke(7462)
	actor := newStubActor(char, newTestRoom())
	first := ExecuteSmother(actor)
	require.True(t, first.Executed)

	second := ExecuteSmother(actor)
	assert.False(t, second.Executed)
	assert.True(t, second.OnCooldown, "the second smother must be refused on the shared cooldown")
	assert.False(t, CommandIsReady(actor, "smother"), "readiness agrees")
}
