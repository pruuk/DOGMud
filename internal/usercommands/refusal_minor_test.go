package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const blankReadItemId = 999994

// `drop gold` with no amount asks how much; it never drops the purse.
func TestDropGoldWithNoAmountAsksHowMuch(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.Items = nil
	user.Character.Gold = 25
	startFloor := room.Gold
	t.Cleanup(func() { room.Gold = startFloor })
	sentTo(user)

	handled, err := Drop("gold", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	assert.Equal(t, 25, user.Character.Gold, "gold unchanged")
	assert.Equal(t, startFloor, room.Gold, "floor unchanged")
	out := sentTo(user)
	assert.Contains(t, out, `Drop how much gold? Try <ansi fg="command">drop <amount> gold</ansi>.`)
	assert.NotContains(t, out, "aren't carrying")
}

func TestDropGoldWithNoGoldSaysSo(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.Items = nil
	user.Character.Gold = 0
	sentTo(user)

	_, err := Drop("gold", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "You don't have any gold to drop.")
}

// A carried item with no text is not "not carrying".
func TestReadCarriedItemWithNoText(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		blankReadItemId: {ItemId: blankReadItemId, Name: "Smooth Stone", NameSimple: "stone", Type: items.Object, Value: 1},
	}))
	user, room := getTestUserAndRoom(t)
	user.Character.Items = nil
	require.True(t, user.Character.StoreItem(items.New(blankReadItemId)))
	sentTo(user)

	_, err := Read("stone", user, room, 0)
	require.NoError(t, err)
	out := sentTo(user)
	assert.Contains(t, out, "There is nothing written on it.")
	assert.NotContains(t, out, "aren't carrying")

	_, err = Read("zzzmissing", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "aren't carrying")
}
