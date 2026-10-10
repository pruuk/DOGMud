package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
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
