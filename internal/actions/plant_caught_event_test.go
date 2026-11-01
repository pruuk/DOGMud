package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// #375 phase 7: a caught plant fires caught_planting on the mark, so its
// tree can answer (the Pocket Dummy mocks the try). An unseen plant fires
// nothing. The contest rolls dice however lopsided the scores, so each test
// tries until the outcome it pins comes up, and every try is checked.
func stubMobEvents(t *testing.T) *[]mobs.EventContext {
	t.Helper()
	var fired []mobs.EventContext
	orig := mobs.MobEventTryMobBehavior
	mobs.MobEventTryMobBehavior = func(id int, ctx mobs.EventContext) bool {
		fired = append(fired, ctx)
		return true
	}
	t.Cleanup(func() { mobs.MobEventTryMobBehavior = orig })
	return &fired
}

func TestPlant_CaughtFiresCaughtPlantingOnTheMark(t *testing.T) {
	fired := stubMobEvents(t)
	mob := newPlantTestMob(testPlantMobInstId, 100000) // sees everything
	mobs.SetInstanceForTest(testPlantMobInstId, mob)
	defer mobs.SetInstanceForTest(testPlantMobInstId, nil)

	for i := 0; i < 200; i++ {
		*fired = nil
		actor := newPlantPlayerActor(1, 1)
		seedPlantItem(actor)
		res := Plant(actor, PlantOptions{TargetMobInstanceId: testPlantMobInstId, ItemNoun: "!1"})
		if res.Detected {
			require.Equal(t, []mobs.EventContext{{EventType: "caught_planting", UserId: 1002}}, *fired)
			return
		}
		require.Empty(t, *fired, "a plant that was not caught fires nothing")
	}
	t.Fatal("no plant was caught in 200 tries")
}

func TestPlant_UnseenFiresNothing(t *testing.T) {
	fired := stubMobEvents(t)
	mob := newPlantTestMob(testPlantMobInstId, 1)
	mobs.SetInstanceForTest(testPlantMobInstId, mob)
	defer mobs.SetInstanceForTest(testPlantMobInstId, nil)

	for i := 0; i < 200; i++ {
		*fired = nil
		actor := newPlantPlayerActor(100000, 8)
		seedPlantItem(actor)
		res := Plant(actor, PlantOptions{TargetMobInstanceId: testPlantMobInstId, ItemNoun: "!1"})
		if res.Succeeded {
			require.Empty(t, *fired)
			return
		}
	}
	t.Fatal("no plant went unseen in 200 tries")
}
