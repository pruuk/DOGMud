package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: craft_missing_tag_storage_test.go
//
// 🐛 Prod defect, owner 2026-09-21. `craft setting` answered "You are
// missing: copper-wire." to a player holding 39 Copper Wire in their bank.
//
// The pull is all-or-nothing by owner ruling, so a shortfall storage cannot
// fully cover moves nothing at all. Every refusal below it was then written
// from crafting.HasIngredients, which only ever sees what is CARRIED and
// names the first short tag in recipe order. chrysalis-setting lists
// copper-wire first, so the player was sent to fetch the one component they
// had in quantity. The real blocker was chrysalis-shard, listed second and
// absent everywhere.
//
// These tests use the shipped recipe's shape: ingredient A listed first and
// sitting in storage, ingredient B listed second and held nowhere.

const craftStorageRecipeId = "test-storage-missing-setting"
const craftStorageRecipeName = "Test Storage Setting"

// craftStorageRecipe mirrors chrysalis-setting: two ingredients, the one the
// bank can supply listed FIRST. No station and no skill floor, so the only
// gate this recipe can fail is the ingredient check.
func craftStorageRecipe() *crafting.RecipeSpec {
	return &crafting.RecipeSpec{
		RecipeId:       craftStorageRecipeId,
		Name:           craftStorageRecipeName,
		Skill:          "jewelcrafting",
		SkillMinimum:   0,
		TimeRounds:     4,
		Output:         crafting.RecipeOutput{ItemId: 10001, Quantity: 1},
		SuccessMessage: "You set the shard.",
		FailureMessage: "The shard splits.",
		Ingredients: []crafting.RecipeIngredient{
			{ItemTag: "copper-wire", Quantity: 1},
			{ItemTag: "chrysalis-shard", Quantity: 1},
		},
	}
}

// craftStorageComponent builds a banked component carrying tag.
func craftStorageComponent(tag string) items.Item {
	return items.Item{
		ItemId: 10001,
		Spec: &items.ItemSpec{
			ItemId:       10001,
			Name:         tag,
			Type:         items.Object,
			IsComponent:  true,
			ComponentTag: tag,
		},
	}
}

// registerCraftStorageRecipe installs the fixture recipe in the global
// registry and teaches it to the user, returning the undo.
func registerCraftStorageRecipe(t *testing.T, user *users.UserRecord) func() {
	t.Helper()
	recipe := craftStorageRecipe()
	crafting.RegisterRecipeForTest(recipe)
	if user.Character.KnownRecipes == nil {
		user.Character.KnownRecipes = map[string]int{}
	}
	user.Character.KnownRecipes[craftStorageRecipeId] = 1
	return func() {
		crafting.UnregisterRecipeForTest(craftStorageRecipeId)
		delete(user.Character.KnownRecipes, craftStorageRecipeId)
	}
}

// TestCraft_RefusalNamesWhatStorageCannotCover is the owner's prod case,
// driven through the real Craft() command.
func TestCraft_RefusalNamesWhatStorageCannotCover(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)
	room.Lamp = rooms.LampPtr(90) // pin fully lit; see combat_blind_warning_test.go in internal/hooks
	undo := registerCraftStorageRecipe(t, user)
	defer undo()

	// The bank holds the wire and nothing else, exactly as the owner's did.
	user.ItemStorage = users.Storage{Slots: []users.StorageSlot{
		{Item: craftStorageComponent("copper-wire"), Count: 39},
	}}
	craftPlainLines(user.UserId)

	handled, err := Craft(craftStorageRecipeName, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	lines := craftPlainLines(user.UserId)
	joined := strings.Join(lines, "\n")
	require.Contains(t, joined, "You are missing:",
		"expected the ingredient refusal, got %v", lines)
	// No item spec carries these fixture tags, so the refusal shows each tag
	// as plain words (items.ComponentTagName, #530), never the dashed id.
	require.Contains(t, joined, "You are missing: chrysalis shard.",
		"the refusal must name what the bank ALSO lacks, got %v", lines)
	require.NotContains(t, joined, "copper wire",
		"the refusal named the component sitting in the player's bank: %v", lines)
	require.NotContains(t, joined, "chrysalis-shard",
		"the refusal showed a dashed component id: %v", lines)
}

// TestRecipeStatus_MissingRowNamesWhatStorageCannotCover covers the other
// player-facing surface: the `craft list` row's reason column.
func TestRecipeStatus_MissingRowNamesWhatStorageCannotCover(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)
	room.Lamp = rooms.LampPtr(90) // pin fully lit; see combat_blind_warning_test.go in internal/hooks
	undo := registerCraftStorageRecipe(t, user)
	defer undo()

	user.ItemStorage = users.Storage{Slots: []users.StorageSlot{
		{Item: craftStorageComponent("copper-wire"), Count: 39},
	}}

	indicator, reason := recipeStatus(user, room, crafting.GetRecipe(craftStorageRecipeId), 0)
	require.Equal(t, "X", indicator, "the recipe is not craftable, so the row is not ready")
	require.Equal(t, "missing chrysalis shard", reason,
		"the row must name what the bank ALSO lacks, not the first-listed tag")
}

// TestRecipeStatus_StorageCompletableStaysReady is the sibling half: when the
// bank CAN cover the shortfall the row must still read ready with no reason,
// because the pull happens automatically at craft time.
func TestRecipeStatus_StorageCompletableStaysReady(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)
	room.Lamp = rooms.LampPtr(90) // pin fully lit; see combat_blind_warning_test.go in internal/hooks
	undo := registerCraftStorageRecipe(t, user)
	defer undo()

	user.ItemStorage = users.Storage{Slots: []users.StorageSlot{
		{Item: craftStorageComponent("copper-wire"), Count: 39},
		{Item: craftStorageComponent("chrysalis-shard"), Count: 2},
	}}

	indicator, reason := recipeStatus(user, room, crafting.GetRecipe(craftStorageRecipeId), 0)
	require.Equal(t, "V", indicator)
	require.Equal(t, "", reason)
}

// TestEnsureComponentsFromStorage_LeavesABusyPlayersBankAlone.
//
// ensureComponentsFromStorage runs before the dispatch, and AlreadyCrafting is
// not returned until actions.InitiateCraft below it. Without a busy guard
// INSIDE the function, a player who is already working has components hauled
// out of the bank onto their person and is then refused anyway.
//
// The guard belongs in the function, not in Craft(): an early return up there
// would skip the pull for every path below it, which is the ordering defect
// the function's own comment block warns about and craft_storage_order_test.go
// guards against.
func TestEnsureComponentsFromStorage_LeavesABusyPlayersBankAlone(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)
	room.Lamp = rooms.LampPtr(90) // pin fully lit; see combat_blind_warning_test.go in internal/hooks
	undo := registerCraftStorageRecipe(t, user)
	defer undo()

	// The bank covers the whole recipe, so nothing but the busy guard can
	// stop the pull.
	user.ItemStorage = users.Storage{Slots: []users.StorageSlot{
		{Item: craftStorageComponent("copper-wire"), Count: 3},
		{Item: craftStorageComponent("chrysalis-shard"), Count: 3},
	}}
	bankedBefore := len(user.ItemStorage.GetItems())
	carriedBefore := len(user.Character.Items) + len(user.Character.ComponentItems)

	user.Character.Activity = activity.NewMachine()
	require.NoError(t, user.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: "something-else", RoundsTotal: 3},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	))
	require.True(t, user.Character.IsCrafting(), "fixture: the player must be busy")
	craftPlainLines(user.UserId)

	ensureComponentsFromStorage(user, room, crafting.GetRecipe(craftStorageRecipeId))

	require.Equal(t, bankedBefore, len(user.ItemStorage.GetItems()),
		"a busy player's bank was raided for a craft that is about to be refused")
	require.Equal(t, carriedBefore, len(user.Character.Items)+len(user.Character.ComponentItems),
		"components were moved onto a busy player who cannot craft them")
	require.Empty(t, craftPlainLines(user.UserId),
		"a busy player was told they drew from storage")
}

// TestCraft_EveryMissingIngredientRefusalIsStorageAware.
//
// There are three player-facing surfaces that name a missing tag: the shared
// InitiateCraft refusal, the enchanting sub-path's own refusal, and the
// `craft list` row. The first two are driven separately above and below this
// file's reach (enchanting needs an equipped target and a real enchantment
// definition), so this pins the property they share at the source: no
// player-facing missing-tag text may be written from a carried-only count.
//
// Same shape as TestCraft_SkillRefusalsUseTheHelper, for the same reason: a
// fourth refusal site added later must not quietly reintroduce the bug.
func TestCraft_EveryMissingIngredientRefusalIsStorageAware(t *testing.T) {
	src := craftSource(t)

	refusals := strings.Count(src, "You are missing: %s.")
	require.Equal(t, 2, refusals,
		"expected the two missing-ingredient refusals (InitiateCraft result and craftEnchanting)")

	helperCalls := strings.Count(src, "storageAwareMissingTag(") -
		strings.Count(src, "func storageAwareMissingTag(")
	require.Equal(t, 2, helperCalls,
		"both missing-ingredient refusals must print storageAwareMissingTag, or one of them "+
			"names a component the player already has banked")

	// The `craft list` row asks the storage-aware primitive directly.
	require.Contains(t, funcBody(t, src, "func recipeStatus("), "crafting.HasIngredientsWithStorage(",
		"the craft list row must count storage when it names a missing tag")
	require.NotContains(t, funcBody(t, src, "func recipeStatus("), "crafting.HasIngredients(",
		"the craft list row still asks the carried-only question")
}

// TestEnsureComponentsFromStorage_BusyGuardMatchesInitiateCraft pins the two
// busy predicates together. If actions.InitiateCraft ever stops using
// IsCrafting for AlreadyCrafting, the guard here has to move with it or a
// busy player's bank gets raided again.
func TestEnsureComponentsFromStorage_BusyGuardMatchesInitiateCraft(t *testing.T) {
	body := funcBody(t, craftSource(t), "func ensureComponentsFromStorage(")
	require.Contains(t, body, "user.Character.IsCrafting()",
		"the busy guard must use the same predicate actions.InitiateCraft uses")

	pull := strings.Index(body, "crafting.PlanStoragePull(")
	guard := strings.Index(body, "user.Character.IsCrafting()")
	require.NotEqual(t, -1, pull, "the storage pull vanished from ensureComponentsFromStorage")
	require.Less(t, guard, pull, "the busy guard must come before anything is pulled")
}
