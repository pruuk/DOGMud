package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// MobVisualSnapshot is VisualSnapshot's decision for the NPCs (#781 ruling
// 14): in a lit room an awake NPC sees fully and a sleeping one sees nothing,
// as a sleeping player reads no room lines. MobSnapshotReaders yields only
// the NPCs in the room now and in the snapshot.
func TestMobVisualSnapshot_SleepersSeeNothing(t *testing.T) {
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const roomId, awakeId, sleeperId, lateId, sleepCond = 99501, 98501, 98502, 98503, 15
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		sleepCond: {ConditionId: sleepCond, Name: "Sleeping", Flags: []conditions.Flag{conditions.Sleeping}, TriggerCount: 1000000},
	}))
	t.Cleanup(SeedRoomsForTest(map[int]*Room{
		roomId: {RoomId: roomId, Zone: "test", Lamp: LampPtr(60)},
	}, map[string]*ZoneConfig{}))
	r := LoadRoom(roomId)
	place := func(id int) *mobs.Mob {
		m := &mobs.Mob{InstanceId: id, Character: *characters.New()}
		m.Character.RoomId = roomId
		mobs.SetInstanceForTest(id, m)
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		r.AddMob(id)
		return m
	}
	place(awakeId)
	sleeper := place(sleeperId)
	require.NoError(t, sleeper.Character.AddCondition(sleepCond, true))
	require.True(t, sleeper.Character.HasConditionFlag(conditions.Sleeping), "fixture: asleep")

	snap := r.MobVisualSnapshot()
	require.Equal(t, MobVisualSnapshot{awakeId: messaging.SightFull, sleeperId: messaging.SightNone}, snap)

	place(lateId) // arrives after the moment
	got := map[int]messaging.SightDecision{}
	for m, d := range r.MobSnapshotReaders(snap) {
		got[m.InstanceId] = d
	}
	require.Equal(t, map[int]messaging.SightDecision{awakeId: messaging.SightFull, sleeperId: messaging.SightNone}, got,
		"only the NPCs in the room now and in the snapshot")
}
