package mobs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #375 phase 7: a caught plant and a foiled ambush are behaviour-tree events
// on the mob, dispatched through MobEventTryMobBehavior (set by behaviortree
// at init, as AttackRejectedTryMobBehavior is). A mob actor passes user 0.
func TestFireCaughtPlantingAndAmbushFoiled_Dispatch(t *testing.T) {
	var ids []int
	var got []EventContext
	orig := MobEventTryMobBehavior
	MobEventTryMobBehavior = func(id int, ctx EventContext) bool {
		ids = append(ids, id)
		got = append(got, ctx)
		return true
	}
	t.Cleanup(func() { MobEventTryMobBehavior = orig })

	m := &Mob{InstanceId: 4242}
	FireCaughtPlanting(m, 7)
	FireAmbushFoiled(m, 0)
	FireCaughtPlanting(nil, 7)
	FireAmbushFoiled(nil, 7)

	require.Equal(t, []int{4242, 4242}, ids)
	require.Equal(t, EventContext{EventType: "caught_planting", UserId: 7}, got[0])
	require.Equal(t, EventContext{EventType: "ambush_foiled", UserId: 0}, got[1])
}
