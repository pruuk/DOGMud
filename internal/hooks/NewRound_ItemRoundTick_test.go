package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

const (
	tickTreedItem   = 999980
	tickFixtureItem = 999981
	tickPlainItem   = 999982
	tickRoomA       = 9980
	tickRoomB       = 9981
	tickMobInst     = 9982
)

// tickCountingTree counts item_idle visits in the item's own state.
const tickCountingTree = "tree:\n  type: action\n  event: item_idle\n  do: increment_state\n  key: visits\n"

// tickFixtureTree lights a fixture at 52 on every visit.
const tickFixtureTree = "tree:\n  type: action\n  event: item_idle\n  do: set_light\n  level: 52\n"

type tickWorld struct {
	user    *users.UserRecord
	mob     *mobs.Mob
	roomA   *rooms.Room
	roomB   *rooms.Room
	worn    items.Item
	pack    items.Item
	mobHeld items.Item
	floor   items.Item
	fixture items.Item
}

// seedTickWorld: user 1 wears a treed item and carries one; mob 9982
// carries one; room A has a treed item on the floor and room B a fixture;
// both rooms are loaded and every holder is indexed.
func seedTickWorld(t *testing.T) *tickWorld {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		tickTreedItem:   {ItemId: tickTreedItem, Name: "test humming ring", Type: items.Ring, Subtype: items.Wearable, Behavior: "tick_count"},
		tickFixtureItem: {ItemId: tickFixtureItem, Name: "test lamp post", Type: items.Object, Fixture: items.FixtureLight, Behavior: "tick_fixture"},
		tickPlainItem:   {ItemId: tickPlainItem, Name: "test pebble", Type: items.Object},
	}))
	behaviortree.LoadItemTreeForTest(t, "tick_count", tickCountingTree)
	behaviortree.LoadItemTreeForTest(t, "tick_fixture", tickFixtureTree)
	t.Cleanup(items.ResetHolderIndexForTest())
	t.Cleanup(itemlight.ResetForTest())
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	prevHook := items.OnRoomHolderIndexed
	items.OnRoomHolderIndexed = nil
	t.Cleanup(func() { items.OnRoomHolderIndexed = prevHook })

	w := &tickWorld{
		worn: items.New(tickTreedItem), pack: items.New(tickTreedItem), mobHeld: items.New(tickTreedItem),
		floor: items.New(tickTreedItem), fixture: items.New(tickFixtureItem),
	}
	w.roomA = rooms.NewRoom("probe")
	w.roomA.RoomId = tickRoomA
	w.roomB = rooms.NewRoom("probe")
	w.roomB.RoomId = tickRoomB
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{tickRoomA: w.roomA, tickRoomB: w.roomB}, map[string]*rooms.ZoneConfig{}))

	w.user = users.NewTestUser(1, "ticker", "Ticker", 0)
	w.user.Character.RoomId = tickRoomA
	w.user.Character.Equipment.Ring = w.worn
	w.user.Character.Items = []items.Item{w.pack, items.New(tickPlainItem)}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: w.user}))

	w.mob = &mobs.Mob{MobId: 1, InstanceId: tickMobInst, HomeRoomId: tickRoomA,
		Character: characters.Character{Name: "Tick Keeper", RoomId: tickRoomA, MobInstanceId: tickMobInst,
			Conditions: conditions.New(), Items: []items.Item{w.mobHeld}}}
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{1: w.mob}, map[int]*mobs.Mob{tickMobInst: w.mob}))

	w.roomA.AddItem(w.floor, false)
	w.roomB.AddItem(w.fixture, false)
	w.mob.Character.IndexTreedItems()

	prevRound := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCountForTest(prevRound) })
	util.SetRoundCountForTest(5000)
	return w
}

func visits(t *testing.T, it items.Item) int {
	t.Helper()
	s := behaviortree.ItemBTreeStateForTest(it.UUID)
	if s == nil {
		return 0
	}
	return s.GetInt("visits")
}

// Rule 5: one tick fires item_idle once for every treed item it reaches:
// a player's worn and backpack items, an indexed mob's, an indexed room's
// floor. A fixture's tree lights it.
func TestItemRoundTickVisitsEveryTreedItemOnce(t *testing.T) {
	w := seedTickWorld(t)
	ItemRoundTick(events.NewRound{})
	for name, it := range map[string]items.Item{"worn": w.worn, "backpack": w.pack, "mob-held": w.mobHeld, "floor": w.floor} {
		if got := visits(t, it); got != 1 {
			t.Errorf("%s item visited %d times, want 1", name, got)
		}
	}
	if v, _ := itemlight.Get(tickRoomB, w.fixture.UUID); v != 52 {
		t.Errorf("the fixture reads %v after a tick, want 52", v)
	}
}

// Rule 5: a holder with nothing treed left, a gone mob and an unloaded room
// drop out of the index; a room that unloads loses its fixtures' output.
func TestItemRoundTickDropsHoldersWithNothingLeft(t *testing.T) {
	w := seedTickWorld(t)
	items.IndexMobHolder(424242) // a mob that no longer exists
	w.roomA.RemoveItem(w.floor, false)
	ItemRoundTick(events.NewRound{})
	if got := items.MobHolders(); len(got) != 1 || got[0] != tickMobInst {
		t.Errorf("MobHolders = %v, want only %d", got, tickMobInst)
	}
	if got := items.RoomHolders(); len(got) != 1 || got[0] != tickRoomB {
		t.Errorf("RoomHolders = %v, want only %d", got, tickRoomB)
	}

	w.mob.Character.Items = nil
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{}, map[string]*rooms.ZoneConfig{})) // room B unloads
	ItemRoundTick(events.NewRound{})
	if got := items.MobHolders(); len(got) != 0 {
		t.Errorf("a mob holding nothing treed is still indexed: %v", got)
	}
	if got := items.RoomHolders(); len(got) != 0 {
		t.Errorf("an unloaded room is still indexed: %v", got)
	}
	if light, _ := itemlight.Terms(tickRoomB); len(light) != 0 {
		t.Errorf("an unloaded room's fixture still lights it: %v", light)
	}
}

// Rule 4: an item's state is evicted once a round passes without a visit,
// and an item handed from a mob to a player keeps its state.
func TestItemStateFollowsTheItem(t *testing.T) {
	w := seedTickWorld(t)
	ItemRoundTick(events.NewRound{})

	// The mob hands its item to the player between rounds.
	w.user.Character.Items = append(w.user.Character.Items, w.mobHeld)
	w.mob.Character.Items = nil
	// The floor item leaves for somewhere the tick does not reach.
	w.roomA.RemoveItem(w.floor, false)

	util.SetRoundCountForTest(5001)
	ItemRoundTick(events.NewRound{})
	if got := visits(t, w.mobHeld); got != 2 {
		t.Errorf("the handed item's visits = %d, want 2: its state followed it", got)
	}
	if s := behaviortree.ItemBTreeStateForTest(w.floor.UUID); s != nil {
		t.Error("an item the tick no longer reaches kept its state")
	}
}

// Rule 5: a fixture is evaluated the moment its room joins the index, so a
// first visit is never dark for a round.
func TestAFixtureIsLitTheMomentItsRoomIsIndexed(t *testing.T) {
	w := seedTickWorld(t)
	items.OnRoomHolderIndexed = EvaluateRoomFixtures
	r := rooms.NewRoom("probe")
	r.RoomId = 9983
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{tickRoomA: w.roomA, tickRoomB: w.roomB, 9983: r}, map[string]*rooms.ZoneConfig{}))
	post := items.New(tickFixtureItem)
	r.AddItem(post, false)
	if v, _ := itemlight.Get(9983, post.UUID); v != 52 {
		t.Errorf("before any tick the new fixture reads %v, want 52", v)
	}
}
