package buffs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ids clear of the other fixtures in this package.
const (
	immunityTestStoneStomachId = 9401
	immunityTestVenomId        = 9402
	immunityTestHarmlessId     = 9403
	immunityTestRealVenomId    = 9404
)

// Stone Stomach's poison-immunity flag was read by nothing: the potion
// promised immunity and delivered a dexterity penalty. Both primitives now
// refuse a poison-flagged spec while it is held.
func TestPoisonImmunityRefusesPoisonBuffs(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		immunityTestStoneStomachId: {BuffId: immunityTestStoneStomachId, Name: "Test Stone Stomach", TriggerCount: 5, RoundInterval: 1, Flags: []Flag{PoisonImmunity}},
		immunityTestVenomId:        {BuffId: immunityTestVenomId, Name: "Test Venom", TriggerCount: 5, RoundInterval: 1, Flags: []Flag{Poison}},
		immunityTestHarmlessId:     {BuffId: immunityTestHarmlessId, Name: "Test Harmless", TriggerCount: 5, RoundInterval: 1},
	})
	defer restore()

	bs := New()
	require.True(t, bs.AddBuff(immunityTestStoneStomachId, false))
	assert.False(t, bs.AddBuff(immunityTestVenomId, false), "a poison buff is refused while immune")
	assert.False(t, bs.HasBuff(immunityTestVenomId))
	assert.False(t, bs.AddBuffScaled(immunityTestVenomId, 0.5), "the scaled primitive refuses too")
	assert.False(t, bs.HasBuff(immunityTestVenomId))
	assert.True(t, bs.AddBuff(immunityTestHarmlessId, false), "a non-poison buff still lands")

	unprotected := New()
	assert.True(t, unprotected.AddBuff(immunityTestVenomId, false), "without immunity poison lands")
}

// The real Venom (buff 39) is a negative health tick, and until this slice it
// carried no flags at all, so CancelBuffsWithFlag(Poison) in Purge Affliction
// and Cleansing Wave matched nothing and the immunity would have refused
// nothing real. A spec shaped like the shipped file must be refused.
func TestPoisonImmunityRefusesAHealthTickVenom(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		immunityTestStoneStomachId: {BuffId: immunityTestStoneStomachId, Name: "Test Stone Stomach", TriggerCount: 5, RoundInterval: 1, Flags: []Flag{PoisonImmunity}},
		immunityTestRealVenomId: {BuffId: immunityTestRealVenomId, Name: "Test Real Venom", TriggerCount: 6, RoundInterval: 1,
			TickPool: "health", TickPercent: -2, TickMin: 1, Flags: []Flag{Poison}},
	})
	defer restore()

	immune := New()
	require.True(t, immune.AddBuff(immunityTestStoneStomachId, false))
	assert.False(t, immune.AddBuff(immunityTestRealVenomId, false), "the venom a mob actually applies is refused")
	assert.False(t, immune.HasBuff(immunityTestRealVenomId))

	unprotected := New()
	assert.True(t, unprotected.AddBuff(immunityTestRealVenomId, false), "without immunity the venom lands")
}
