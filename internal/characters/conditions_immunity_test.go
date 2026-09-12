package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const conditionsImmunityStoneStomachId = 9411

// Poison reaches a character two ways: a poison-flagged buff and the poisoned
// condition, which two spell sites add directly. Stone Stomach must stop both,
// or an immune drinker still reads the poisoned adjective and takes the tick.
func TestPoisonImmunityRefusesThePoisonedCondition(t *testing.T) {
	restore := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		conditionsImmunityStoneStomachId: {BuffId: conditionsImmunityStoneStomachId, Name: "Test Stone Stomach",
			TriggerCount: 5, RoundInterval: 1, Flags: []buffs.Flag{buffs.PoisonImmunity}},
	})
	defer restore()

	c := &Character{Buffs: buffs.New()}
	require.True(t, c.Buffs.AddBuff(conditionsImmunityStoneStomachId, false))

	c.AddCondition(ConditionPoisoned, 5, 1, "test")
	assert.False(t, c.HasCondition(ConditionPoisoned), "the poisoned condition is refused while immune")

	c.AddCondition(ConditionBlinded, 5, 1, "test")
	assert.True(t, c.HasCondition(ConditionBlinded), "every other condition still lands")

	unprotected := &Character{Buffs: buffs.New()}
	unprotected.AddCondition(ConditionPoisoned, 5, 1, "test")
	assert.True(t, unprotected.HasCondition(ConditionPoisoned), "without immunity the condition lands")
}
