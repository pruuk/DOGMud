package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: craft_instant_room_hiding_test.go
//
// The instant-craft narration (case result.ImmediateComplete in Craft(), and
// completeCraft, the enchanting instant path). Both share one delivery tail
// (craftDeliverInstant): a self-only success line to the crafter, and the
// room's start and success lines through actions.NarrateCraftPhase (messaging
// M6 row 7), read from the crafting messages store. The sender owns name
// hiding; these tests pin that a shapes-only third party reads both room
// lines with the crafter hidden and the item as "something".
//
// Aliceia (user 1) crafts; Bobrick (user 2) is the shapes-only bystander.

const craftHidingInfraredConditionId = 9101

// craftHidingNightVisionConditionId grants the CRAFTER clear sight in the
// unlit fixture room.
//
// Crafting refuses a maker who cannot see clearly (owner ruling 2026-09-21,
// craft.go), so the scenario these tests pin is no longer "everyone is in the
// dark". It is the sharper one: the crafter CAN see, the bystander can make
// out warmth only, and the bystander must still read "a figure" rather than a
// name. Without this the craft is refused and nothing is narrated at all.
//
// GRADED LIGHTING PLAN 2. Under the window model no shift can reach SightFull
// at light 0 with the SHIPPED band edges: even the shift cap (24) cannot
// clear LightDimBelow's default of 50. TestCraftImmediateComplete_RealBranch_
// ShapesOnlyThirdPartyReadsAFigure (the one caller that needs the crafter at
// SightFull here) declares this condition's nightvision_strength at the cap
// AND pins a smaller LightBlindBelow/LightDimBelow pair for its own scope
// (configs.SetConfigForTest), so the crafter's shifted window clears the
// room's light while the bystander's infra reach still only reaches shapes.
// That is a real, valid server configuration, not a fabricated one: a lower
// LightDimBelow is exactly the kind of thing this arc's config knobs exist
// to let an operator tune.
const craftHidingNightVisionConditionId = 9102

// seedCraftHidingCondition installs a test condition that grants
// InfraredVision, additive on top of seedAllRegistries' condition registry.
// Call it AFTER `defer cleanup := seedAllRegistries()` and defer its own
// result so it restores BEFORE the fixture does (SeedConditionsForTest
// replaces the whole registry).
func seedCraftHidingCondition() func() {
	return conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		// GRADED LIGHTING PLAN 2: a bare InfraredVision flag reads reach 0 by
		// design (internal/characters/vision.go), so this fixture declares
		// an explicit infra_reach, matching shipped condition 85, or the
		// watcher would stop being shapes-only in the dark at all.
		craftHidingInfraredConditionId: {
			ConditionId: craftHidingInfraredConditionId,
			Name:        "Test Heat Eyes",
			Flags:       []conditions.Flag{conditions.InfraredVision},
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}},
		},
		craftHidingNightVisionConditionId: {
			ConditionId: craftHidingNightVisionConditionId,
			Name:        "Test Dark Eyes",
			Flags:       []conditions.Flag{conditions.NightVision},
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectNightVisionStrength: {Literal: 24}},
		},
	})
}

// darkenCraftRoom turns a fixture room into an unlit cave, so CanSeeClearly
// fails and a bystander with InfraredVision sight lands at SightShapes.
//
// Room 1 carries a room-level Lamp pin (lighting plan 5b's shop sight gate
// needs it lit; see seedAllRegistries) that overrides a biome switch, same
// trap noted in combat_fire_test.go: a room-level Lamp is not a biome
// default, so it survives switching Biome to "cave" unless cleared first.
func darkenCraftRoom(t *testing.T, roomId int) {
	t.Helper()
	room := rooms.LoadRoom(roomId)
	require.NotNil(t, room)
	room.Lamp = nil
	room.Biome = "cave"
	require.Equal(t, 0, room.LightLevel(), "room %d must actually be unlit", roomId)
}

var craftHidingTagPattern = regexp.MustCompile(`<[^>]*>`)

// craftPlainLines drains a user's queued messages and returns them tag-stripped.
func craftPlainLines(userId int) []string {
	raw := events.DrainQueuedMessagesForTest(userId)
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		out = append(out, strings.TrimSpace(craftHidingTagPattern.ReplaceAllString(line, "")))
	}
	return out
}

func craftCountContaining(lines []string, needle string) int {
	n := 0
	for _, l := range lines {
		if strings.Contains(l, needle) {
			n++
		}
	}
	return n
}

// craftInstantHidingRecipe returns a minimal recipe for the two real-branch
// regression tests below: no station, no skill floor, no ingredients, so the
// only thing under test is the delivery of its narration.
func craftInstantHidingRecipe(id string) *crafting.RecipeSpec {
	return &crafting.RecipeSpec{
		RecipeId:       id,
		Name:           id,
		Skill:          "test-craft-hiding-skill",
		TimeRounds:     0,
		Output:         crafting.RecipeOutput{ItemId: 10001, Quantity: 1},
		SuccessMessage: "You finish your work.",
	}
}

// seedCraftHidingMessages seeds the craft room-line pools for the hiding
// recipe's skill. The room lines come from the store, not the recipe file.
func seedCraftHidingMessages(t *testing.T) {
	t.Helper()
	t.Cleanup(crafting.SeedCraftingMessagesForTest(crafting.CraftingMessages{"test-craft-hiding-skill": {
		StartObserver:   []string{"{actor} begins {item}."},
		SuccessObserver: []string{"{actor} finishes {item}."},
		FailureObserver: []string{"{actor} ruins it."},
		StartHeard:      []string{"You hear work close by."},
		FinishHeard:     []string{"The work stops."},
	}}))
}

// TestCompleteCraft_RealBranch_ShapesOnlyThirdPartyReadsAFigure is a
// regression guard: it drives the real completeCraft function (the
// enchanting instant-complete path) with a shapes-only third party in the
// room, and pins that the room reads the store's start and success lines in
// order, crafter hidden, item "something", and the crafter reads neither.
func TestCompleteCraft_RealBranch_ShapesOnlyThirdPartyReadsAFigure(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	darkenCraftRoom(t, 1)

	crafter := users.GetByUserId(1)
	watcher := users.NewTestUser(3, "cara", "Carrow", 1003)
	watcher.Character.RoomId = 1
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: crafter, 2: users.GetByUserId(2), 3: watcher})
	defer restoreUsers()
	room := rooms.LoadRoom(1)
	room.AddPlayer(3)
	require.True(t, watcher.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))
	seedCraftHidingMessages(t)
	craftPlainLines(1)
	craftPlainLines(3)

	recipe := craftInstantHidingRecipe("test-complete-craft-hiding")
	completeCraft(crafter, room, recipe)

	crafterLines, watcherLines := craftPlainLines(1), craftPlainLines(3)
	require.Equal(t, 1, craftCountContaining(crafterLines, "You finish your work."))
	require.Equal(t, []string{"A figure begins something.", "A figure finishes something."}, watcherLines)
	require.Equal(t, 0, craftCountContaining(watcherLines, "Aliceia"))
	require.Equal(t, 0, craftCountContaining(crafterLines, "begins"), "the crafter reads no room line")
}

// TestCraftImmediateComplete_RealBranch_ShapesOnlyThirdPartyReadsAFigure is
// the same regression guard for the OTHER instant-craft site: case
// result.ImmediateComplete inside Craft() itself, reached through
// actions.InitiateCraft with a zero-time-rounds recipe. Same two lines as
// completeCraft's test.
func TestCraftImmediateComplete_RealBranch_ShapesOnlyThirdPartyReadsAFigure(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	darkenCraftRoom(t, 1)

	// GRADED LIGHTING PLAN 2: with the SHIPPED band edges (LightBlindBelow
	// 25, LightDimBelow 50) no shift can reach SightFull at light 0, since
	// even the shift cap (24) cannot clear 50. This scenario needs the
	// crafter to see clearly AND the bystander to stay shapes-only in the
	// SAME dark room, so this test pins a smaller band pair, scoped to this
	// test only, under which the crafter's capped shift (see
	// craftHidingNightVisionConditionId) clears the room's light while the
	// bystander's infra reach (unshifted) still only reaches shapes.
	cfg := configs.GetConfig()
	cfg.Balance.LightBlindBelow = 10
	cfg.Balance.LightDimBelow = 20
	configs.SetConfigForTest(t, cfg)

	crafter := users.GetByUserId(1)
	watcher := users.NewTestUser(3, "cara", "Carrow", 1003)
	watcher.Character.RoomId = 1
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: crafter, 2: users.GetByUserId(2), 3: watcher})
	defer restoreUsers()
	room := rooms.LoadRoom(1)
	room.AddPlayer(3)
	require.True(t, watcher.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))
	// The crafter must see clearly: Craft() refuses a maker who cannot.
	require.True(t, crafter.Character.Conditions.AddCondition(craftHidingNightVisionConditionId, true))

	recipe := craftInstantHidingRecipe("test-immediate-complete-hiding")
	crafting.RegisterRecipeForTest(recipe)
	defer crafting.UnregisterRecipeForTest(recipe.RecipeId)
	crafter.Character.KnownRecipes = map[string]int{recipe.RecipeId: 1}

	seedCraftHidingMessages(t)
	craftPlainLines(1)
	craftPlainLines(3)

	handled, err := Craft(recipe.Name, crafter, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	crafterLines, watcherLines := craftPlainLines(1), craftPlainLines(3)
	require.Equal(t, 1, craftCountContaining(crafterLines, "You finish your work."))
	require.Equal(t, []string{"A figure begins something.", "A figure finishes something."}, watcherLines)
	require.Equal(t, 0, craftCountContaining(watcherLines, "Aliceia"))
	require.Equal(t, 0, craftCountContaining(crafterLines, "begins"), "the crafter reads no room line")
}

// TestCraft_RefusedWhenTheMakerCannotSee pins owner ruling 2026-09-21: an
// ATTEMPT to make something is refused when the maker cannot see clearly.
//
// Two tiers are refused, not one. Total darkness is the obvious case. Infrared
// is the interesting one: it is enough to tell that a shape is moving, which
// is why a bystander with it still reads "a figure" in the tests above, and it
// is NOT enough to do fine work. That is why the gate reads CanSeeClearly
// rather than CanSeeShapes.
//
// Listing recipes stays allowed, because knowing what you can make is memory
// rather than sight.
func TestCraft_RefusedWhenTheMakerCannotSee(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	darkenCraftRoom(t, 1)

	crafter := users.GetByUserId(1)
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: crafter, 2: users.GetByUserId(2)})
	defer restoreUsers()
	room := rooms.LoadRoom(1)

	recipe := craftInstantHidingRecipe("test-refused-when-blind")
	crafting.RegisterRecipeForTest(recipe)
	defer crafting.UnregisterRecipeForTest(recipe.RecipeId)
	crafter.Character.KnownRecipes = map[string]int{recipe.RecipeId: 1}

	t.Run("no sight at all is refused", func(t *testing.T) {
		craftPlainLines(1)
		handled, err := Craft(recipe.Name, crafter, room, 0)
		require.NoError(t, err)
		require.True(t, handled)

		lines := craftPlainLines(1)
		require.Equal(t, 1, craftCountContaining(lines, "You can't see well enough to work on anything here."),
			"a maker in the dark must be refused, got: %v", lines)
		require.Equal(t, 0, craftCountContaining(lines, "You finish your work."),
			"the craft must not have happened")
	})

	t.Run("shapes only is refused too", func(t *testing.T) {
		require.True(t, crafter.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))
		craftPlainLines(1)
		handled, err := Craft(recipe.Name, crafter, room, 0)
		require.NoError(t, err)
		require.True(t, handled)

		lines := craftPlainLines(1)
		require.Equal(t, 1, craftCountContaining(lines, "You can't see well enough to work on anything here."),
			"making out warm shapes is not enough for fine work, got: %v", lines)
	})

	t.Run("listing what you know is still allowed", func(t *testing.T) {
		craftPlainLines(1)
		handled, err := Craft("list", crafter, room, 0)
		require.NoError(t, err)
		require.True(t, handled)

		lines := craftPlainLines(1)
		require.Equal(t, 0, craftCountContaining(lines, "You can't see well enough to work on anything here."),
			"listing recipes is memory, not sight, got: %v", lines)
	})
}
