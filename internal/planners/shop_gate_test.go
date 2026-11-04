package planners

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/itemvalue"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/stretchr/testify/require"
)

// A mob's pathto ignores a room_gate, so a planner must not send a mob to a shop
// or a station behind one: thief mob 9318 would otherwise walk through Marn's
// cupboard door to Sefton to sell what it stole. A gate guards a room whatever
// its conditions, because a mob holds no quest tokens to read them against.

const (
	gatedShopRoom = 9901
	openShopRoom  = 9902
	gatedShopItem = 99010
)

func registerRoomGate(t *testing.T, room int) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999641,
		Name:    "Shop Gate Quest",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:      "room_gate",
			Room:       room,
			Conditions: questengine.Conditions{Missing: []string{"999641-start"}},
			Actions:    []questengine.ActionDef{{SendText: "No."}},
		}},
	})
}

func gateShopMob(zone string) *mobs.Mob {
	m := &mobs.Mob{}
	m.Character.Zone = zone
	return m
}

func gateShop(zone string, mobId, room int) {
	shops.RegisterShop(zone, mobId, room, shops.ShopInventory{Gold: 1000, Stock: []shops.StockEntry{
		{ItemId: gatedShopItem, RestockQty: 1, MaxStock: 5},
	}})
}

func TestShopFinders_SkipAGatedRoom(t *testing.T) {
	registerRoomGate(t, gatedShopRoom)
	gateShop("GateShop Only", 1, gatedShopRoom)
	only := gateShopMob("GateShop Only")

	_, ok := findShopInZoneBuying(only)
	require.False(t, ok, "the only shop is behind the gate")
	_, ok = findShopInZoneSelling(only, "", gatedShopItem)
	require.False(t, ok, "the only shop is behind the gate")

	gateShop("GateShop Both", 1, gatedShopRoom)
	gateShop("GateShop Both", 2, openShopRoom)
	both := gateShopMob("GateShop Both")
	for range 20 { // AllShops comes from a map, so its order changes by run
		room, ok := findShopInZoneBuying(both)
		require.True(t, ok)
		require.Equal(t, openShopRoom, room)
		room, ok = findShopInZoneSelling(both, "", gatedShopItem)
		require.True(t, ok)
		require.Equal(t, openShopRoom, room)
	}

	// With no gate on file the same shop is an ordinary shop.
	t.Cleanup(questengine.ResetEngineForTest())
	room, ok := findShopInZoneBuying(only)
	require.True(t, ok, "an ungated shop is found")
	require.Equal(t, gatedShopRoom, room)
}

func TestScanZoneUpgrades_SkipsAGatedRoom(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		gatedShopItem: {ItemId: gatedShopItem, Name: "Test Blade", Type: items.Weapon, Subtype: items.Slashing, Value: 10},
	}))
	gateShop("GateShop Upgrade", 1, gatedShopRoom)
	mob := gateShopMob("GateShop Upgrade")
	profile := itemvalue.WeightProfile{}

	_, ok := scanZoneUpgrades(mob, profile, 100000, false, -1000)
	require.True(t, ok, "fixture: with no gate the blade is an upgrade")

	registerRoomGate(t, gatedShopRoom)
	_, ok = scanZoneUpgrades(mob, profile, 100000, false, -1000)
	require.False(t, ok, "a shop behind a gate offers no upgrade")
}

func TestFindCraftingStation_SkipsAGatedRoom(t *testing.T) {
	const zone = "GateStation"
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{gatedShopRoom: {RoomId: gatedShopRoom, Zone: zone, Station: "forge"}},
		map[string]*rooms.ZoneConfig{zone: {Name: zone, RoomId: gatedShopRoom, RoomIds: map[int]struct{}{gatedShopRoom: {}}}},
	))
	mob := gateShopMob(zone)
	room, ok := findCraftingStationInZone(mob, "forge")
	require.True(t, ok, "fixture: with no gate the forge is found")
	require.Equal(t, gatedShopRoom, room)

	registerRoomGate(t, gatedShopRoom)
	_, ok = findCraftingStationInZone(mob, "forge")
	require.False(t, ok, "a forge behind a gate is out of reach")
}
