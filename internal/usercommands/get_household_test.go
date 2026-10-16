package usercommands

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A plain `get` of a household's bauble is refused, with the steal command
// named, and nothing is taken, now through the shared floor pickup
// (actions.ErrHouseholdBauble; owner ruling 2026-09-29).
func TestHouseholdBauble_GetRefusesThroughTheSharedPickup(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	// Midsummer noon, so get is not refused as blind (look_item_noun_test.go).
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	util.SetRoundCountForTest(uint64(3430))
	t.Cleanup(util.ResetRoundCountForTest)

	restoreItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {ItemId: items.BaubleItemId, Name: "Curious Trinket", NameSimple: "trinket",
			Type: items.Object, Subtype: items.Mundane, Weight: 0.2, Value: 1, NotSalable: true},
	})
	defer restoreItems()
	baubles.SetDirForTest(t.TempDir())
	defer items.SetBaubleResolver(nil)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Strength.ValueAdj = 50
	origItems := user.Character.Items
	user.Character.Items = nil
	defer func() { user.Character.Items = origItems }()

	rec, err := baubles.Create(baubles.Record{Name: "Small Child's Doll", NameSimple: "doll", Tier: baubles.TierCheap,
		Value: 3, WeightLbs: 0.5, Description: "A rag doll with one button eye.", Status: baubles.StatusReady})
	require.NoError(t, err)
	theirs := items.New(items.BaubleItemId)
	theirs.Bauble = rec.Id
	theirs.LeaveBaubleAt("on the shelf", room.RoomId, time.Now())
	room.AddItem(theirs, false)
	defer room.RemoveItem(theirs, false)

	events.DrainQueuedMessagesForTest(user.UserId)
	handled, err := Get("doll", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	assert.Contains(t, out, "belongs to this household. Try")
	assert.Empty(t, user.Character.Items, "nothing taken")
}
