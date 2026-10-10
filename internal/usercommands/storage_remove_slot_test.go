package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #308: a lone number is a slot only for remove; `storage add 5` still asks
// "add what?" rather than hunting for an item called 5.
func TestStorageAdd_LoneNumberAsksWhat(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	oldIsStorage := room.IsStorage
	room.IsStorage = true
	origStorage := user.ItemStorage
	t.Cleanup(func() {
		room.IsStorage = oldIsStorage
		user.ItemStorage = origStorage
	})
	user.ItemStorage = users.Storage{}
	events.DrainQueuedMessagesForTest(user.UserId)

	_, err := Storage("add 5", user, room, 0)
	require.NoError(t, err)

	got := sentTo(user)
	assert.NotContains(t, got, "You don't have a")
	assert.Contains(t, strings.ToLower(got), "add what?")
}

// #308: `storage remove 2` read the 2 as a quantity with no item name and
// answered "You don't have a  in storage." It takes slot 2.
func TestStorageRemove_BySlotNumber(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	oldIsStorage := room.IsStorage
	room.IsStorage = true
	origStorage, origItems := user.ItemStorage, user.Character.Items
	origStrength := user.Character.Stats.Strength.ValueAdj
	t.Cleanup(func() {
		room.IsStorage = oldIsStorage
		user.ItemStorage, user.Character.Items = origStorage, origItems
		user.Character.Stats.Strength.ValueAdj = origStrength
	})
	user.ItemStorage = users.Storage{}
	user.Character.Items = nil
	user.Character.Stats.Strength.ValueAdj = 50
	require.True(t, user.ItemStorage.AddItem(items.New(10001)))
	require.True(t, user.ItemStorage.AddItem(items.New(20001)))
	want := user.ItemStorage.GetSlots()[1].Item.ItemId
	events.DrainQueuedMessagesForTest(user.UserId)

	_, err := Storage("remove 2", user, room, 0)
	require.NoError(t, err)

	assert.NotContains(t, sentTo(user), "You don't have a")
	require.Len(t, user.Character.Items, 1, "slot 2 comes out")
	assert.Equal(t, want, user.Character.Items[0].ItemId)
	assert.Equal(t, 1, user.ItemStorage.SlotCount())
}
