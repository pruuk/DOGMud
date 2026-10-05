package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A darkness fixture does not read as "lit" while it darkens the room.
func TestLookDarknessFixtureWording(t *testing.T) {
	user, room := seedFixtureRoom(t)
	useDogmudTemplates(t)
	t.Cleanup(itemlight.ResetForTest())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		fixtureCmdItemId: {ItemId: fixtureCmdItemId, Name: "Void Brazier", NameSimple: "brazier", Type: items.Object,
			Fixture: items.FixtureDarkness, Value: 1, NotSalable: true},
		pebbleCmdItemId: {ItemId: pebbleCmdItemId, Name: "Grey Pebble", NameSimple: "pebble", Type: items.Object,
			Weight: 0.1, Value: 1},
	}))

	Look("", user, room, 0)
	assert.Contains(t, sentTo(user), `The <ansi fg="item">Void Brazier</ansi> is still.`)

	b, ok := room.FindOnFloor("brazier", false)
	require.True(t, ok)
	itemlight.Set(room.RoomId, b.UUID, itemlight.Darkness, 30)
	Look("", user, room, 0)
	out := sentTo(user)
	assert.Contains(t, out, `The <ansi fg="item">Void Brazier</ansi> swallows the light.`)
	assert.NotContains(t, out, "is lit.")
}
