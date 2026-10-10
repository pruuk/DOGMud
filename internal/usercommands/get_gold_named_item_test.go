package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goldWireItemId = 999981

// seedGoldWireRoom is seedFixtureRoom (get_fixture_test.go) with a Gold Wire
// on the floor and no gold pile. It seeds its own item registry because
// items.SeedItemsForTest REPLACES the registry rather than adding to it.
func seedGoldWireRoom(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	util.SetRoundCountForTest(uint64(3430)) // midsummer noon: nothing refused as blind
	t.Cleanup(util.ResetRoundCountForTest)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		goldWireItemId: {ItemId: goldWireItemId, Name: "Gold Wire", NameSimple: "wire", Type: items.Object,
			Weight: 0.1, Value: 1},
	}))
	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Strength.ValueAdj = 50
	// grantCorpseGold sends corpse gold to the party pool when the looter is in
	// a party, so a party left over from an earlier test would eat the gold.
	if p := parties.Get(user.UserId); p != nil {
		p.Disband()
	}
	t.Cleanup(func() {
		if p := parties.Get(user.UserId); p != nil {
			p.Disband()
		}
	})
	origGold := user.Character.Gold
	t.Cleanup(func() { user.Character.Gold = origGold })
	origItems := user.Character.Items
	user.Character.Items = nil
	t.Cleanup(func() { user.Character.Items = origItems })
	wire := items.New(goldWireItemId)
	room.AddItem(wire, false)
	t.Cleanup(func() { room.RemoveItem(wire, false) })
	oldGold := room.Gold
	room.Gold = 0
	t.Cleanup(func() { room.Gold = oldGold })
	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

// #269: a floor item whose name starts with "Gold" was read as the gold pile,
// so `get gold wire` and `get all` both answered "There's no gold to grab."
func TestGet_ItemNamedGoldIsPickedUp(t *testing.T) {
	user, room := seedGoldWireRoom(t)
	_, err := Get("gold wire", user, room, 0)
	require.NoError(t, err)
	assert.NotContains(t, sentTo(user), "There's no gold to grab.")
	_, carried := user.Character.FindInBackpack("gold wire")
	assert.True(t, carried, "get gold wire picks up the Gold Wire")
}

// `get all` reaches the same item by its name.
func TestGetAll_PicksUpAnItemNamedGold(t *testing.T) {
	user, room := seedGoldWireRoom(t)
	Get("all", user, room, 0)
	assert.NotContains(t, sentTo(user), "There's no gold to grab.")
	_, carried := user.Character.FindInBackpack("gold wire")
	assert.True(t, carried, "get all picks up the Gold Wire")
}

// Plain `get gold` still means the gold pile.
func TestGet_GoldStillMeansThePile(t *testing.T) {
	user, room := seedGoldWireRoom(t)
	Get("gold", user, room, 0)
	assert.Contains(t, sentTo(user), "There's no gold to grab.")
}

// `get gold from ground` is the same pile; "from ground" is stripped first.
func TestGet_GoldFromGroundStillMeansThePile(t *testing.T) {
	user, room := seedGoldWireRoom(t)
	room.Gold = 15
	user.Character.Gold = 0
	Get("gold from ground", user, room, 0)
	assert.Equal(t, 15, user.Character.Gold)
	assert.Equal(t, 0, room.Gold)
}

// The floor pile is taken into the purse, not just announced.
func TestGet_GoldPileRaisesPurse(t *testing.T) {
	user, room := seedGoldWireRoom(t)
	room.Gold = 40
	user.Character.Gold = 0
	Get("gold", user, room, 0)
	assert.Equal(t, 40, user.Character.Gold)
	assert.Equal(t, 0, room.Gold)
}

// seedGoldWireChest puts a Gold Wire and goldInside gold in a chest and in a
// corpse's loot, on top of seedGoldWireRoom.
func seedGoldWireChest(t *testing.T, goldInside int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	user, room := seedGoldWireRoom(t)
	origContainers, origCorpses := room.Containers, room.Corpses
	t.Cleanup(func() { room.Containers, room.Corpses = origContainers, origCorpses })
	room.Containers = map[string]rooms.Container{
		"chest": {Gold: goldInside, Items: []items.Item{items.New(goldWireItemId)}},
	}
	room.Corpses = []rooms.Corpse{{
		MobId:     1,
		Character: characters.Character{Name: "Skeleton"},
		Loot:      rooms.Container{Gold: goldInside, Items: []items.Item{items.New(goldWireItemId)}},
	}}
	user.Character.Gold = 0
	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

func TestGet_GoldNamedItemFromContainer(t *testing.T) {
	user, room := seedGoldWireChest(t, 0)
	Get("gold wire from chest", user, room, 0)
	assert.NotContains(t, sentTo(user), "There's no gold to grab.")
	_, carried := user.Character.FindInBackpack("gold wire")
	assert.True(t, carried, "get gold wire from chest picks up the Gold Wire")
	assert.Empty(t, room.Containers["chest"].Items)
}

func TestGetAll_ContainerTakesItemNamedGold(t *testing.T) {
	user, room := seedGoldWireChest(t, 0)
	Get("all chest", user, room, 0)
	_, carried := user.Character.FindInBackpack("gold wire")
	assert.True(t, carried, "get all chest picks up the Gold Wire")
	assert.Empty(t, room.Containers["chest"].Items)
}

func TestGet_GoldFromContainerStillTakesGold(t *testing.T) {
	user, room := seedGoldWireChest(t, 25)
	Get("gold from chest", user, room, 0)
	assert.Equal(t, 25, user.Character.Gold)
	assert.Equal(t, 0, room.Containers["chest"].Gold)
	assert.Len(t, room.Containers["chest"].Items, 1, "the Gold Wire stays put")
}

func TestGet_GoldNamedItemFromCorpse(t *testing.T) {
	user, room := seedGoldWireChest(t, 25)
	Get("gold wire from skeleton corpse", user, room, 0)
	_, carried := user.Character.FindInBackpack("gold wire")
	assert.True(t, carried, "get gold wire from corpse picks up the Gold Wire")
	assert.Equal(t, 0, user.Character.Gold, "the corpse gold is not taken")
	assert.Equal(t, 25, room.Corpses[0].Loot.Gold)
}

func TestGet_GoldFromCorpseStillTakesGold(t *testing.T) {
	user, room := seedGoldWireChest(t, 25)
	Get("gold from skeleton corpse", user, room, 0)
	assert.Equal(t, 25, user.Character.Gold)
}
