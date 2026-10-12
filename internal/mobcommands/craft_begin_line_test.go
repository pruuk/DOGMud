package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A mob that starts a timed craft says what it is making.
func TestCraftBeginLineNamesTheItem(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		8301: {ItemId: 8301, Name: "iron dagger"},
	}))

	line := craftBeginLine("Brann", &crafting.RecipeSpec{Output: crafting.RecipeOutput{ItemId: 8301}})
	require.Contains(t, line, `<ansi fg="mobname">Brann</ansi> begins working on `)
	require.Contains(t, line, `<ansi fg="itemname">Iron Dagger</ansi>.`)

	require.Contains(t, craftBeginLine("Brann", &crafting.RecipeSpec{}), "begins working on something.")
	require.Contains(t, craftBeginLine("Brann", nil), "begins working on something.")
}

// The item name reaches only a reader who sees clearly. A shapes-only reader
// reads "something" about "a figure"; a reader who sees nothing reads nothing.
func TestAnnounceCraftBeginJudgesEachReader(t *testing.T) {
	cases := []struct {
		name  string
		lit   bool
		heat  bool
		sight messaging.SightDecision
		want  []string
	}{
		{"full sight reads the item", true, false, messaging.SightFull,
			[]string{"Skeleton begins working on Iron Dagger."}},
		{"shapes reads something", false, true, messaging.SightShapes,
			[]string{"A figure begins working on something."}},
		{"no sight reads nothing", false, false, messaging.SightNone, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
				8301: {ItemId: 8301, Name: "iron dagger"},
			}))
			t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
				"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", SkyLight: rooms.SkyLightPtr(0.0), MovementCost: 1},
			}))
			t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
				9001: {ConditionId: 9001, Name: "Test Infrared", RoundInterval: 1, TriggerCount: 1, Flags: []conditions.Flag{conditions.InfraredVision},
					Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
			}))
			mob, room := getTestMobAndRoom(t)
			origBiome, origSky, origLamp := room.Biome, room.SkyLight, room.Lamp
			t.Cleanup(func() { room.Biome, room.SkyLight, room.Lamp = origBiome, origSky, origLamp })
			room.Biome, room.SkyLight, room.Lamp = "cave", rooms.SkyLightPtr(0), nil
			if tc.lit {
				room.Lamp = rooms.LampPtr(60)
			}
			reader := users.GetByUserId(1)
			if tc.heat {
				require.True(t, reader.Character.Conditions.AddCondition(9001, true))
			}
			room.AddPlayer(1)
			require.Equal(t, tc.sight, messaging.ReaderSight(reader.Character, room), "fixture: reader sight")
			events.DrainQueuedMessagesForTest(1)

			announceCraftBegin(room, mob.Character.Name,
				&crafting.RecipeSpec{Output: crafting.RecipeOutput{ItemId: 8301}})

			require.Equal(t, tc.want, mobSpeechHeard(1))
		})
	}
}
