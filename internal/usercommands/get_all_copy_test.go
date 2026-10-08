package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #409: a bare `get all` on a floor with nothing to take printed nothing at
// all. A fixture is not something to take, so a floor holding only the Arch
// Lantern counts as empty.
func TestGetAllOnAnEmptyFloorSaysSo(t *testing.T) {
	user, room := seedFixtureRoom(t)
	Get("pebble", user, room, 0)
	require.Len(t, user.Character.Items, 1, "fixture: the pebble is taken first")
	sentTo(user)
	room.Gold = 0

	_, err := Get("all", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "There is nothing here to pick up.")
}

// A floor with something on it does not get the empty line.
func TestGetAllWithSomethingToTakeIsNotEmpty(t *testing.T) {
	user, room := seedFixtureRoom(t)
	room.Gold = 0
	_, err := Get("all", user, room, 0)
	require.NoError(t, err)
	assert.NotContains(t, sentTo(user), "nothing here to pick up")
}

// #409: "You pick up 1 item(s)." read machine-made. The count picks the noun.
func TestGetAllNamedCountsItemsInWords(t *testing.T) {
	user, room := seedFixtureRoom(t)
	_, err := Get("all pebble", user, room, 0)
	require.NoError(t, err)
	out := sentTo(user)
	assert.Contains(t, out, "You pick up 1 item.")
	assert.NotContains(t, out, "item(s)")

	two := []items.Item{items.New(pebbleCmdItemId), items.New(pebbleCmdItemId)}
	for _, p := range two {
		room.AddItem(p, false)
	}
	_, err = Get("all pebble", user, room, 0)
	require.NoError(t, err)
	out = sentTo(user)
	assert.Contains(t, out, "You pick up 2 items.")
	assert.NotContains(t, out, "item(s)")
}

// `drop all <name>` is the sweep's mirror and counts the same way.
func TestDropAllNamedCountsItemsInWords(t *testing.T) {
	user, room := seedFixtureRoom(t)
	user.Character.Items = []items.Item{items.New(pebbleCmdItemId)}
	_, err := Drop("all pebble", user, room, 0)
	require.NoError(t, err)
	out := sentTo(user)
	assert.Contains(t, out, "You drop 1 item.")
	assert.NotContains(t, out, "item(s)")

	user.Character.Items = []items.Item{items.New(pebbleCmdItemId), items.New(pebbleCmdItemId)}
	_, err = Drop("all pebble", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "You drop 2 items.")
}

// The component-bag sweep says the same in words.
func TestGetAllBagCountsItemsInWords(t *testing.T) {
	user, room := seedFixtureRoom(t)
	origComp := user.Character.ComponentItems
	t.Cleanup(func() { user.Character.ComponentItems = origComp })

	user.Character.ComponentItems = []items.Item{items.New(pebbleCmdItemId)}
	_, err := Get("all bag", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "You move 1 item from your component bag to your backpack.")

	user.Character.ComponentItems = []items.Item{items.New(pebbleCmdItemId), items.New(pebbleCmdItemId)}
	_, err = Get("all bag", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "You move 2 items from your component bag to your backpack.")
}
