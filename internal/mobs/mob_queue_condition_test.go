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

// The four older mob doors all go through QueueCondition and still queue
// exactly what they did: the mob's id and life epoch, the door's own fields,
// and no caster.
func TestMob_OldDoorsQueueNoCaster(t *testing.T) {
	const instanceId = 88104
	m := &Mob{InstanceId: instanceId}
	m.Character.LifeEpoch = 6
	events.DrainQueuedMobConditionsForTest(instanceId)

	m.AddCondition(5, "potion")
	m.AddConditionScaled(7, 2.5, "drink")
	m.AddConditionMagnitude(119, 4, 9, "spell")
	m.AddConditionTickScaled(32, 1.5, "spell")

	queued := events.DrainQueuedMobConditionsForTest(instanceId)
	require.Len(t, queued, 4)
	// The queue is a priority heap, so find each event by its condition id.
	byId := map[int]events.Condition{}
	for _, e := range queued {
		assert.Zero(t, e.UserId, "condition %d", e.ConditionId)
		assert.Equal(t, instanceId, e.MobInstanceId, "condition %d", e.ConditionId)
		assert.Equal(t, uint64(6), e.LifeEpoch, "condition %d", e.ConditionId)
		assert.True(t, e.Caster.IsZero(), "condition %d", e.ConditionId)
		byId[e.ConditionId] = e
	}
	require.Len(t, byId, 4)

	assert.Equal(t, "potion", byId[5].Source)

	assert.Equal(t, "drink", byId[7].Source)
	assert.Equal(t, 2.5, byId[7].DurationMult)

	assert.Equal(t, "spell", byId[119].Source)
	assert.Equal(t, 4, byId[119].Triggers)
	assert.Equal(t, 9.0, byId[119].Magnitude)

	assert.Equal(t, "spell", byId[32].Source)
	assert.Equal(t, 1.5, byId[32].TickScale)
}
