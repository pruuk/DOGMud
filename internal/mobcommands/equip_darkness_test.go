package mobcommands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Ruling D6 on the mob path, the sibling of the player's equip line: a mob
// putting on a darkness is announced judged as lit, so the players it has
// just blinded still see it happen.
func TestMobEquippingADarknessIsSeenBeforeTheDarkFalls(t *testing.T) {
	const umbralCond, umbralItem = 9771, 999971
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		umbralCond: {ConditionId: umbralCond, Name: "Test Umbral Dark", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 50}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		umbralItem: {ItemId: umbralItem, Name: "test umbral lantern", NameSimple: "lantern",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{umbralCond}},
	}))
	mob, room := getTestMobAndRoom(t)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = rooms.LampPtr(50)
	require.True(t, mob.Character.StoreItem(items.Item{ItemId: umbralItem}))
	events.DrainQueuedMessagesForTest(1)

	_, err := Equip(fmt.Sprintf("!%d", umbralItem), mob, room)
	require.NoError(t, err)
	require.Equal(t, umbralItem, mob.Character.Equipment.Light.ItemId, "fixture: the lantern must be worn")
	require.Less(t, room.LightLevel(), 25, "fixture: the lantern must have taken the room below normal sight")
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(1), "\n"), "puts on",
		"the player the mob's lantern just blinded missed it being put on")
}
