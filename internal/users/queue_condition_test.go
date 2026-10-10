package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// QueueCondition is the one door that carries a caster to the apply hook
// (messaging M6 slice 1, section 1): the producer fills the event, the door
// stamps the holder and the life epoch, whatever the producer wrote there.
func TestUserRecord_QueueConditionStampsTheHolderAndKeepsTheCaster(t *testing.T) {
	const userId = 9932
	events.DrainQueuedConditionsForTest(userId)

	u := &UserRecord{UserId: userId, Character: &characters.Character{LifeEpoch: 4}}
	u.QueueCondition(events.Condition{
		UserId: 1, MobInstanceId: 77, LifeEpoch: 99, // overwritten by the door
		ConditionId: 38, Source: "spell", Magnitude: 12, Triggers: 5,
		Caster: state.ActorRef{UserId: 3}, CasterCrit: true,
	})

	queued := events.DrainQueuedConditionsForTest(userId)
	require.Len(t, queued, 1)
	got := queued[0]
	assert.Equal(t, userId, got.UserId)
	assert.Zero(t, got.MobInstanceId)
	assert.Equal(t, uint64(4), got.LifeEpoch)
	assert.Equal(t, state.ActorRef{UserId: 3}, got.Caster)
	assert.True(t, got.CasterCrit)
	assert.Equal(t, 12.0, got.Magnitude)
	assert.Equal(t, 5, got.Triggers)
	assert.Equal(t, "spell", got.Source)
}

// The older doors still queue exactly what they did, with no caster.
func TestUserRecord_OldDoorsQueueNoCaster(t *testing.T) {
	const userId = 9933
	events.DrainQueuedConditionsForTest(userId)
	u := &UserRecord{UserId: userId}
	u.AddCondition(5, "potion")
	u.AddConditionMagnitude(119, 4, 9, "spell")
	u.AddConditionTickScaled(32, 1.5, "spell")

	queued := events.DrainQueuedConditionsForTest(userId)
	require.Len(t, queued, 3)
	for _, e := range queued {
		assert.True(t, e.Caster.IsZero(), "condition %d", e.ConditionId)
	}
	assert.Equal(t, 1.5, queued[2].TickScale)
	assert.Equal(t, 9.0, queued[1].Magnitude)
}
