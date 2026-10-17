package usercommands

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A household's bauble: `get` refuses it and names the steal command, and
// `steal` finds it (only this household's) by any word of its name.
func TestHouseholdBauble_GetRefusesAndStealFindsIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {ItemId: items.BaubleItemId, Name: "Curious Trinket", NameSimple: "trinket",
			Type: items.Object, Subtype: items.Mundane, Weight: 0.2, Value: 1, NotSalable: true},
	})
	defer restoreItems()
	baubles.SetDirForTest(t.TempDir())
	defer items.SetBaubleResolver(nil)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Strength.ValueAdj = 50

	rec, err := baubles.Create(baubles.Record{
		Name: "Small Child's Doll", NameSimple: "doll", Tier: baubles.TierCheap, Value: 3, WeightLbs: 0.5,
		Description: "A rag doll with one button eye.", Status: baubles.StatusReady,
	})
	require.NoError(t, err)
	theirs := items.New(items.BaubleItemId)
	theirs.Bauble = rec.Id
	theirs.LeaveBaubleAt("on the shelf", room.RoomId, time.Now())
	room.AddItem(theirs, false)
	defer room.RemoveItem(theirs, false)

	got, ok := householdBaubleNamed(room, "small doll")
	require.True(t, ok)
	assert.Equal(t, rec.Id, got.Bauble)
	assert.Equal(t, "doll", stealWord(theirs))

	// A bauble anyone dropped is nobody's household's: `get` takes that one.
	nobodys := items.New(items.BaubleItemId)
	nobodys.Bauble = rec.Id
	room.RemoveItem(theirs, false)
	room.AddItem(nobodys, false)
	defer room.RemoveItem(nobodys, false)
	_, ok = householdBaubleNamed(room, "doll")
	assert.False(t, ok, "steal only takes a household's bauble")
	room.AddItem(theirs, false)

	// The steal command resolves the words to the household's bauble.
	opts := parseStealArgs([]string{"small", "doll"}, room, user)
	require.NotNil(t, opts)
	assert.Equal(t, rec.Id, opts.HouseholdItem.Bauble)
}

// #553: in a room too dark to `get`, `steal <bauble>` cannot find a
// household's bauble on the floor, and answers exactly as for a name that
// matches nothing, so a thief cannot probe the dark floor for names.
func TestHouseholdBauble_StealFindsNothingInTheDark(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {ItemId: items.BaubleItemId, Name: "Curious Trinket", NameSimple: "trinket",
			Type: items.Object, Subtype: items.Mundane, Weight: 0.2, Value: 1, NotSalable: true},
	})
	defer restoreItems()
	baubles.SetDirForTest(t.TempDir())
	defer items.SetBaubleResolver(nil)

	user, room := getTestUserAndRoom(t)
	rec, err := baubles.Create(baubles.Record{
		Name: "Small Child's Doll", NameSimple: "doll", Tier: baubles.TierCheap, Value: 3, WeightLbs: 0.5,
		Description: "A rag doll with one button eye.", Status: baubles.StatusReady,
	})
	require.NoError(t, err)
	theirs := items.New(items.BaubleItemId)
	theirs.Bauble = rec.Id
	theirs.LeaveBaubleAt("on the shelf", room.RoomId, time.Now())
	room.AddItem(theirs, false)
	defer room.RemoveItem(theirs, false)

	room.Lamp = nil
	room.Biome = "cave"
	require.True(t, actions.TooDarkToGet(&actions.UserActor{User: user, Room: room}), "precondition: too dark to get")
	events.DrainQueuedMessagesForTest(user.UserId)

	assert.Nil(t, parseStealArgs([]string{"small", "doll"}, room, user), "a household bauble was named in the dark")
	hit := events.DrainQueuedMessagesForTest(user.UserId)
	assert.Nil(t, parseStealArgs([]string{"no", "such", "thing"}, room, user))
	miss := events.DrainQueuedMessagesForTest(user.UserId)
	assert.Equal(t, miss, hit, "a bauble that is there answers as a name that is not")
}

// `get all trinket` with a household's trinket listed first and the
// player's own dropped one after it: the household's is left (and said so),
// and the sweep goes on to take the player's.
func TestHouseholdBauble_GetAllSkipsItAndTakesTheRest(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {ItemId: items.BaubleItemId, Name: "Curious Trinket", NameSimple: "trinket",
			Type: items.Object, Subtype: items.Mundane, Weight: 0.2, Value: 1, NotSalable: true},
	})
	defer restoreItems()
	baubles.SetDirForTest(t.TempDir())
	defer items.SetBaubleResolver(nil)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Strength.ValueAdj = 50
	user.Character.Items = nil

	silver, err := baubles.Create(baubles.Record{Name: "Silver Trinket", NameSimple: "trinket", Tier: baubles.TierCheap,
		Value: 3, WeightLbs: 0.2, Description: "A silver charm.", Status: baubles.StatusReady})
	require.NoError(t, err)
	brass, err := baubles.Create(baubles.Record{Name: "Brass Trinket", NameSimple: "trinket", Tier: baubles.TierCheap,
		Value: 2, WeightLbs: 0.2, Description: "A brass charm.", Status: baubles.StatusReady})
	require.NoError(t, err)

	theirs := items.New(items.BaubleItemId)
	theirs.Bauble = silver.Id
	theirs.LeaveBaubleAt("on the shelf", room.RoomId, time.Now())
	mine := items.New(items.BaubleItemId)
	mine.Bauble = brass.Id
	room.Items = nil
	room.AddItem(theirs, false)
	room.AddItem(mine, false)
	defer func() { room.Items = nil }()

	getAllMatchingFromFloor(user, room, "trinket")

	require.Len(t, room.Items, 1, "one left on the floor")
	assert.Equal(t, silver.Id, room.Items[0].Bauble, "the household's stays")
	found := false
	for _, it := range user.Character.Items {
		if it.Bauble == brass.Id {
			found = true
		}
	}
	assert.True(t, found, "the player's own is taken, past the household's")
}
