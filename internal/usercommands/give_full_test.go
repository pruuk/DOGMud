package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const heavyGiveItemId = 999993

// A recipient who cannot carry the item is a refusal ("They can't carry any
// more."), not a server fault, and the item stays with the giver.
func TestGive_RecipientOverCapacityIsRefused(t *testing.T) {
	setup := func(t *testing.T) *users.UserRecord {
		t.Cleanup(seedAllRegistries())
		t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
			heavyGiveItemId: {ItemId: heavyGiveItemId, Name: "Anvil", NameSimple: "anvil", Type: items.Object, Weight: 500, Value: 1},
		}))
		user, _ := getTestUserAndRoom(t)
		user.Character.Stats.Strength.ValueAdj = 100000
		user.Character.Items = nil
		require.True(t, user.Character.StoreItem(items.New(heavyGiveItemId)))
		return user
	}
	check := func(t *testing.T, user *users.UserRecord) {
		out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
		assert.Contains(t, out, refusal.RecipientFullLine)
		assert.NotContains(t, out, refusal.FailedText(""), "not a server fault")
		assert.Equal(t, 1, countItemsById(user.Character.Items, heavyGiveItemId), "item stays with the giver")
	}

	t.Run("player", func(t *testing.T) {
		user := setup(t)
		_, room := getTestUserAndRoom(t)
		other := users.GetByUserId(2)
		other.Character.Stats.Strength.ValueAdj = 1
		other.Character.Items = nil
		events.DrainQueuedMessagesForTest(user.UserId)

		handled, err := Give("anvil bobrick", user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		check(t, user)
		assert.Equal(t, 0, countItemsById(other.Character.Items, heavyGiveItemId))
	})

	t.Run("mob", func(t *testing.T) {
		user := setup(t)
		_, room := getTestUserAndRoom(t)
		_, mobInstanceId := room.FindByName("skeleton")
		mob := mobs.GetInstance(mobInstanceId)
		require.NotNil(t, mob)
		mob.Character.Stats.Strength.ValueAdj = 1
		mob.Character.Items = nil
		events.DrainQueuedMessagesForTest(user.UserId)

		handled, err := Give("anvil skeleton", user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		check(t, user)
		assert.Equal(t, 0, countItemsById(mob.Character.Items, heavyGiveItemId))
	})
}
