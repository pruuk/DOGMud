package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/stretchr/testify/require"
)

// X10: a mob idling over a floor fixture never takes it, by the idle floor
// equip (EquipBestFloorItem) or by `get all`, even when it would score as an
// upgrade.
func TestAMobNeverTakesAFloorFixture(t *testing.T) {
	const fixtureItem = 999972
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		fixtureItem: {ItemId: fixtureItem, Name: "test bolted lamp", NameSimple: "lamp",
			Type: items.Light, Subtype: items.Wearable, Fixture: items.FixtureLight,
			PhysicalMitigation: 20}, // would score as an upgrade
	}))
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.BehaviorArchetype = "generic_fighter"
	mob.Character.Awareness = awareness.NewMachine() // the equip path reveals the wearer
	room := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, room)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = rooms.LampPtr(60)
	room.Items = []items.Item{items.New(fixtureItem)}
	t.Cleanup(func() { room.Items = nil })

	require.False(t, EquipBestFloorItem(mob, room), "the mob took a fixture to wear")
	mobcommands.Get("all", mob, room)
	require.Len(t, room.Items, 1, "the fixture left the floor")
	require.Zero(t, mob.Character.Equipment.Light.ItemId)
	require.Empty(t, mob.Character.Items)
}
