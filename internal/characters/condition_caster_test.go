package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The synchronous magnitude door carries a caster too: the spell dot and the
// combat bleeds land through it, never through the event (spec section 1).
func TestAddConditionMagnitudeBy_StampsTheCasterAndTheSource(t *testing.T) {
	defer conditions.SeedConditionRecordsForTest()()
	c := pinCharacter()
	caster := state.ActorRef{UserId: 12}

	require.NoError(t, c.AddConditionMagnitudeBy(conditions.ConditionIdPoisoned, 5, -3, "spell", caster))
	got := c.GetConditions(conditions.ConditionIdPoisoned)
	require.Len(t, got, 1)
	assert.Equal(t, caster, got[0].Caster)
	assert.Equal(t, "spell", got[0].Source)

	// The old door is the same door with no caster: the newest application
	// owns the record, so a casterless re-application leaves none.
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdPoisoned, 5, -3, "potion"))
	assert.True(t, c.GetConditions(conditions.ConditionIdPoisoned)[0].Caster.IsZero())
}

// A stacking record (a bleed) is one record, so its caster is its newest
// applier's.
func TestAddConditionMagnitudeBy_AStackTakesTheNewestCaster(t *testing.T) {
	defer conditions.SeedConditionRecordsForTest()()
	c := pinCharacter()
	require.NoError(t, c.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, 3, -2, "rake", state.ActorRef{MobInstanceId: 4}))
	require.NoError(t, c.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, 3, -2, "maul", state.ActorRef{UserId: 9}))
	got := c.GetConditions(conditions.ConditionIdBleeding)
	require.Len(t, got, 1)
	assert.Len(t, got[0].Stacks, 2)
	assert.Equal(t, state.ActorRef{UserId: 9}, got[0].Caster)
}
