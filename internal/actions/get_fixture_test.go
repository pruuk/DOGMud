package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const getFixtureItemId = 999990

// X10 and Rule 10: a fixture is refused to every taker that picks up
// through TakeFloorItem (a player's get, a mob's, a companion's). Nothing
// moves.
func TestGetItemFromFloor_RefusesAFixtureToEveryTaker(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		getFixtureItemId: {ItemId: getFixtureItemId, Name: "Arch Lantern", NameSimple: "lantern", Type: items.Object,
			Fixture: items.FixtureLight},
	}))
	char := newTestChar()
	room := newTestRoom()
	room.RoomId = 9702
	room.Items = append(room.Items, items.New(getFixtureItemId))
	actor := newStubActor(char, room)

	result := GetItemFromFloor(actor, "lantern", false)
	require.True(t, result.Found, "the lantern is found")
	require.ErrorIs(t, result.Err, ErrFixture)
	assert.Equal(t, 0, countCharItems(char), "nothing taken")
	assert.Equal(t, 1, countFloorItems(room), "the lantern stays fixed")
}

const getOilLanternItemId = 999989

// A fixture first in floor order must not shadow a takeable item the same
// word names: `get lantern` takes the oil lantern.
func TestGetItemFromFloor_PrefersATakeableOverAShadowingFixture(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		getFixtureItemId: {ItemId: getFixtureItemId, Name: "Arch Lantern", NameSimple: "lantern", Type: items.Object,
			Fixture: items.FixtureLight},
		getOilLanternItemId: {ItemId: getOilLanternItemId, Name: "Oil Lantern", NameSimple: "lantern", Type: items.Object},
	}))
	char := newTestChar()
	room := newTestRoom()
	room.RoomId = 9703
	room.Items = append(room.Items, items.New(getFixtureItemId), items.New(getOilLanternItemId))
	actor := newStubActor(char, room)

	result := GetItemFromFloor(actor, "lantern", false)
	require.True(t, result.Found)
	require.NoError(t, result.Err)
	assert.Equal(t, getOilLanternItemId, result.Item.ItemId)
	assert.Equal(t, 1, countCharItems(char), "the oil lantern is taken")
	assert.Equal(t, 1, countFloorItems(room), "the fixture stays")
}
