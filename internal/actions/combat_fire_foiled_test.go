package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #375 phase 7: a same-room shot from hiding that misses fires ambush_foiled
// on the target mob, so long as the mob lived through it. A shot that lands,
// or an ordinary shot, fires nothing.
func TestFireAmbushFoiled_OnlyAMissedShotFromCover(t *testing.T) {
	fired := stubMobEvents(t)
	mob := newStealTestMob(testMobInstId, 0, 1)
	mob.Character.Health = 10
	mobs.SetInstanceForTest(testMobInstId, mob)
	defer mobs.SetInstanceForTest(testMobInstId, nil)
	actor := newStealPlayerActor(100, 1) // user 1001

	fireAmbushFoiled(actor, testMobInstId, true, true)   // landed
	fireAmbushFoiled(actor, testMobInstId, false, false) // ordinary miss
	fireAmbushFoiled(actor, 0, true, false)              // a player target
	require.Empty(t, *fired)

	fireAmbushFoiled(actor, testMobInstId, true, false)
	require.Equal(t, []mobs.EventContext{{EventType: "ambush_foiled", UserId: 1001}}, *fired)

	*fired = nil
	mob.Character.Health = 0
	fireAmbushFoiled(actor, testMobInstId, true, false)
	require.Empty(t, *fired, "a defended shot that still killed the mob is no foiled ambush")
}

// ExecuteFire itself reaches fireAmbushFoiled, once a shot, on the real dice: a
// shot from hiding at a very quick mob until one misses, each try checked.
func TestExecuteFire_RaisesAmbushFoiledOnAMissFromCover(t *testing.T) {
	pinRangedSurpriseBalance(t)
	_, cleanup := seedFireMobInRoom(t, 1, 100000)
	defer cleanup()
	fired := stubMobEvents(t)

	for i := 0; i < 300; i++ {
		*fired = nil
		res := ExecuteFire(newStubActor(newSurpriseShooter(true), rooms.LoadRoom(1)), "skeleton")
		require.True(t, res.Executed)
		if res.MoveResult.Hit {
			require.Empty(t, *fired, "a shot that landed fires nothing")
			continue
		}
		require.Len(t, *fired, 1, "a missed shot from cover fires once")
		require.Equal(t, "ambush_foiled", (*fired)[0].EventType)
		return
	}
	t.Fatal("no shot missed in 300 tries")
}
