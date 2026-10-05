package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Ruling D6 on the floor-loot path, the sibling of the mob equip command: a
// mob that picks a darkness off the floor and dons it is announced judged by
// the room before it went on, so the players it has just blinded still see it
// happen.
func TestMobDonningAFloorDarknessIsSeenBeforeTheDarkFalls(t *testing.T) {
	const umbralCond, umbralItem = 9771, 999971
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		umbralCond: {ConditionId: umbralCond, Name: "Test Umbral Dark", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 50}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		umbralItem: {ItemId: umbralItem, Name: "test umbral lantern", NameSimple: "lantern",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{umbralCond},
			PhysicalMitigation: 20}, // scores as an upgrade, so the mob picks it
	}))
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.BehaviorArchetype = "generic_fighter"
	room := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, room)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = rooms.LampPtr(50)
	room.Items = []items.Item{{ItemId: umbralItem}}
	drainPlain(1)

	require.True(t, EquipBestFloorItem(mob, room), "fixture: the mob must choose the lantern")
	require.Equal(t, umbralItem, mob.Character.Equipment.Light.ItemId, "fixture: the lantern must be worn")
	require.Less(t, room.LightLevel(), 25, "fixture: the lantern must have taken the room below normal sight")
	require.Contains(t, strings.Join(drainPlain(1), "\n"), "picks up",
		"the player the mob's lantern just blinded missed it being donned")
}

// Owner rule, 2026-10-05, on the floor-loot path: judged against the room
// before the lantern goes on, a player already blind in a pitch-dark room
// learns nothing of the mob donning it.
func TestMobDonningAFloorDarknessIsNotSeenInARoomAlreadyDark(t *testing.T) {
	const umbralCond, umbralItem = 9771, 999971
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		umbralCond: {ConditionId: umbralCond, Name: "Test Umbral Dark", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 50}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		umbralItem: {ItemId: umbralItem, Name: "test umbral lantern", NameSimple: "lantern",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{umbralCond},
			PhysicalMitigation: 20},
	}))
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.BehaviorArchetype = "generic_fighter"
	room := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, room)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = nil
	require.Less(t, room.LightLevel(), 25, "fixture: the room must be dark before the lantern")
	room.Items = []items.Item{{ItemId: umbralItem}}
	drainPlain(1)

	require.True(t, EquipBestFloorItem(mob, room), "fixture: the mob must choose the lantern")
	require.Equal(t, umbralItem, mob.Character.Equipment.Light.ItemId, "fixture: the lantern must be worn")
	got := strings.Join(drainPlain(1), "\n")
	require.NotContains(t, got, mob.Character.Name, "a player already blind in the dark was told who donned the lantern")
	require.NotContains(t, got, "picks up", "a player already blind in the dark saw the lantern donned")
}
