package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// stock_item keeps the acting mob carrying an item (#375 phase 7): the
// Pothole Coulee's Pocket Dummy refills the practice purse a student lifted.
const (
	stockItemId     = 975501
	stockPotionId   = 975504
	stockBeltId     = 975505
	stockTemplateId = 975502
	stockInstanceId = 975503
	stockRoomId     = 975590
)

func seedStockMob(t *testing.T) *mobs.Mob {
	t.Helper()
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		stockItemId:   {ItemId: stockItemId, Name: "sand purse", Type: items.Object, Weight: 0.1},
		stockPotionId: {ItemId: stockPotionId, Name: "test tonic", Type: items.Potion, Weight: 0.1},
		stockBeltId: {ItemId: stockBeltId, Name: "test bandolier", Type: items.Belt,
			IsBandolier: true, BandolierCapacity: 5},
	}))
	t.Cleanup(seedTestRoom(t, stockRoomId, "Testzone"))
	t.Cleanup(seedTestMob(t, stockTemplateId, stockInstanceId, stockRoomId, "pocket dummy"))
	m := mobs.GetInstance(stockInstanceId)
	require.NotNil(t, m)
	m.Character.Stats.Strength.ValueAdj = 100 // carry capacity for the purse
	return m
}

func carried(m *mobs.Mob, itemId int) int {
	n := 0
	for _, it := range m.Character.GetAllCarriedItems() {
		if it.ItemId == itemId {
			n++
		}
	}
	return n
}

func TestActStockItem_GivesOneToAnEmptyPocket(t *testing.T) {
	m := seedStockMob(t)
	ctx := &EvalContext{InstanceId: stockInstanceId}

	require.Equal(t, Success, actStockItem(map[string]any{"item_id": stockItemId}, ctx))
	require.Equal(t, 1, carried(m, stockItemId))
}

func TestActStockItem_FailsWhenAlreadyStocked(t *testing.T) {
	m := seedStockMob(t)
	ctx := &EvalContext{InstanceId: stockInstanceId}

	require.Equal(t, Success, actStockItem(map[string]any{"item_id": stockItemId}, ctx))
	require.Equal(t, Failure, actStockItem(map[string]any{"item_id": stockItemId}, ctx),
		"a full pocket fails, so a sequence after it says nothing")
	require.Equal(t, 1, carried(m, stockItemId))
}

func TestActStockItem_MaxAllowsMore(t *testing.T) {
	m := seedStockMob(t)
	ctx := &EvalContext{InstanceId: stockInstanceId}
	params := map[string]any{"item_id": stockItemId, "max": 2}

	require.Equal(t, Success, actStockItem(params, ctx))
	require.Equal(t, Success, actStockItem(params, ctx))
	require.Equal(t, Failure, actStockItem(params, ctx))
	require.Equal(t, 2, carried(m, stockItemId))
}

func TestActStockItem_NoMobOrNoItemFails(t *testing.T) {
	seedStockMob(t)
	require.Equal(t, Failure, actStockItem(map[string]any{"item_id": stockItemId}, &EvalContext{}))
	require.Equal(t, Failure, actStockItem(map[string]any{}, &EvalContext{InstanceId: stockInstanceId}))
}

// StoreItem routes a potion into a worn bandolier, so a count of the backpack
// alone would never see it and every idle tick would add another.
func TestActStockItem_CountsEveryCarryList(t *testing.T) {
	m := seedStockMob(t)
	m.Character.Equipment.Belt = items.New(stockBeltId)
	ctx := &EvalContext{InstanceId: stockInstanceId}
	params := map[string]any{"item_id": stockPotionId}

	require.Equal(t, Success, actStockItem(params, ctx))
	require.Len(t, m.Character.PotionItems, 1, "the fixture must route to the bandolier")
	require.Equal(t, Failure, actStockItem(params, ctx))
	require.Len(t, m.Character.PotionItems, 1)
}

// The Pocket Dummy's shipped tree: an idle tick refills an empty pocket and
// never piles a second purse on a full one, and no other event refills it.
func TestPocketDummyTreeRefillsOneSandPurse(t *testing.T) {
	const sandPurseId = 60
	m := seedStockMob(t)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		sandPurseId: {ItemId: sandPurseId, Name: "Sand Purse", Type: items.Object, Weight: 0.1},
	}))
	t.Cleanup(util.SetRandForTest(1))

	tree, err := LoadTreeFromFile("../../_datafiles/world/dogmud/behaviors/pothole_coulee/9127-pocket_dummy.yaml")
	require.NoError(t, err)

	eval := func(event string) Result {
		return tree.Evaluate(&EvalContext{
			Event:      EventContext{EventType: event, RoomId: stockRoomId},
			MobState:   EnsureBTreeState(m),
			InstanceId: stockInstanceId,
			RoomId:     stockRoomId,
		})
	}

	for i := 0; i < 200; i++ {
		eval("player_enter")
	}
	require.Equal(t, 0, carried(m, sandPurseId), "only an idle tick refills")

	refilledAfter := 0
	for i := 1; i <= 200 && refilledAfter == 0; i++ {
		if eval("mob_idle") == Success {
			refilledAfter = i
		}
	}
	require.NotZero(t, refilledAfter, "two hundred idle ticks at one in five must refill")
	require.Equal(t, 1, carried(m, sandPurseId))

	for i := 0; i < 200; i++ {
		require.NotEqual(t, Success, eval("mob_idle"), "a full pocket says nothing")
	}
	require.Equal(t, 1, carried(m, sandPurseId), "a full pocket gains nothing")
}

func TestActStockItem_ADeadMobGainsNothing(t *testing.T) {
	m := seedStockMob(t)
	m.Character.Life = life.NewMachine()
	require.NoError(t, m.Character.Life.TransitionToDead(life.DeadData{}, state.TransitionReason{}))

	require.Equal(t, Failure, actStockItem(map[string]any{"item_id": stockItemId},
		&EvalContext{InstanceId: stockInstanceId}))
	require.Equal(t, 0, carried(m, stockItemId))
}

// A charmed mob is someone's companion: it restocks nothing for its master.
func TestActStockItem_ACharmedMobGainsNothing(t *testing.T) {
	m := seedStockMob(t)
	m.Character.Charmed = characters.NewCharm(1, characters.CharmPermanent, "")

	require.Equal(t, Failure, actStockItem(map[string]any{"item_id": stockItemId},
		&EvalContext{InstanceId: stockInstanceId}))
	require.Equal(t, 0, carried(m, stockItemId))
}
