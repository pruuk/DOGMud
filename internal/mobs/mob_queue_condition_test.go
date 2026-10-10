package mobs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mob twin of UserRecord.QueueCondition: the door stamps the mob as the
// holder and its life epoch, and the caster rides through.
func TestMobQueueConditionStampsTheHolderAndKeepsTheCaster(t *testing.T) {
	m := &Mob{InstanceId: 88103}
	m.Character.LifeEpoch = 2
	events.DrainQueuedMobConditionsForTest(88103)

	m.QueueCondition(events.Condition{
		UserId: 5, LifeEpoch: 40, // overwritten by the door
		ConditionId: 2, Source: "spell", Caster: state.ActorRef{UserId: 6},
	})

	got := events.DrainQueuedMobConditionsForTest(88103)
	require.Len(t, got, 1)
	assert.Zero(t, got[0].UserId)
	assert.Equal(t, 88103, got[0].MobInstanceId)
	assert.Equal(t, uint64(2), got[0].LifeEpoch)
	assert.Equal(t, state.ActorRef{UserId: 6}, got[0].Caster)
}
