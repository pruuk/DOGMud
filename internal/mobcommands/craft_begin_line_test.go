package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func seedMobCraftMessages(t *testing.T) {
	t.Helper()
	t.Cleanup(crafting.SeedCraftingMessagesForTest(crafting.CraftingMessages{"blacksmithing": {
		StartObserver:   []string{"{actor} begins working on {item}."},
		SuccessObserver: []string{"{actor} finishes {item}."},
		FailureObserver: []string{"{actor} ruins the work."},
		StartHeard:      []string{"You hear hammering close by."},
		FinishHeard:     []string{"The hammering stops."},
	}}))
}

// The item name reaches only a reader who sees clearly. A shapes-only reader
// reads "something" about "a figure"; a reader who sees nothing hears the
// heard line. mobSpeechHeard drops only what the deafen filter drops, and
// the reader is not deafened, so the heard line is returned.
func TestMobCraftStartJudgesEachReader(t *testing.T) {
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
		{"no sight hears the hammering", false, false, messaging.SightNone,
			[]string{"You hear hammering close by."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			seedMobCraftMessages(t)
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

			actions.NarrateCraftPhase(actions.NewMobActorInRoom(mob, room),
				&crafting.RecipeSpec{Skill: "blacksmithing", Output: crafting.RecipeOutput{ItemId: 8301}},
				crafting.PhaseStart, room)

			require.Equal(t, tc.want, mobSpeechHeard(1))
		})
	}
}

// The real mob `craft` command tells a clear-sighted room what it started.
// This is what proves the Initiated site is wired to the shared sender.
func TestMobCraftCommandStartLineReachesRoom(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	seedMobCraftMessages(t)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		8301: {ItemId: 8301, Name: "iron dagger"},
	}))
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{
		RecipeId: "test-iron-dagger", Name: "test iron dagger", Skill: "blacksmithing",
		TimeRounds: 3, Output: crafting.RecipeOutput{ItemId: 8301, Quantity: 1},
	})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest("test-iron-dagger") })
	mob, room := getTestMobAndRoom(t)
	origSky, origLamp := room.SkyLight, room.Lamp
	origKnown, origActivity := mob.Character.KnownRecipes, mob.Character.Activity
	t.Cleanup(func() {
		room.SkyLight, room.Lamp = origSky, origLamp
		mob.Character.KnownRecipes, mob.Character.Activity = origKnown, origActivity
	})
	mob.Character.Activity = activity.NewMachine()
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	mob.Character.KnownRecipes = map[string]int{"test-iron-dagger": 1}
	room.AddPlayer(1)
	require.Equal(t, messaging.SightFull, messaging.ReaderSight(users.GetByUserId(1).Character, room), "fixture: reader sight")
	events.DrainQueuedMessagesForTest(1)

	handled, err := Craft("test iron dagger", mob, room)
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, mob.Character.IsCrafting(), "fixture: the craft must have started")

	require.Equal(t, []string{"Skeleton begins working on Iron Dagger."}, mobSpeechHeard(1))
}
