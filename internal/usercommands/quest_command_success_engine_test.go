package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/require"
)

// The 2026-10-17 newbie quest audit (S3) found three more quest steps that
// advanced on a refusal or a failure: sleep (quest 42's rest step) sent its
// `command` event when the sleep was refused in combat, search (49, 58) sent
// it on a cooldown refusal, and craft (35, 38, 42, 54, 55) sent it when the
// work BEGAN, so a failed brew still granted quest 38's brew step. Owner rule:
// a quest step advances only when the action succeeds. The craft completion
// half (the timed path, resolved in the round tick) is pinned in
// internal/hooks/craft_quest_event_test.go.

func TestSleep_QuestCommandEventOnlyWhenThePlayerFallsAsleep(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		15: {
			ConditionId:  15,
			Name:         "Sleeping",
			Flags:        []conditions.Flag{conditions.Sleeping, conditions.CancelOnDamage},
			TriggerCount: 1000000,
		},
	})
	defer restoreCond()
	user, room := getTestUserAndRoom(t)
	user.Character.Conditions = conditions.New()
	t.Cleanup(func() {
		user.Character.EndAggro()
		user.Character.Conditions = conditions.New()
	})

	user.Character.SetAggro(0, 100, characters.DefaultAttack)
	require.True(t, user.Character.IsInCombat(), "fixture: the sleeper is fighting")
	fired := listenForCommand(t, user, 999241, "sleep")
	_, err := Sleep("", user, room, 0)
	require.NoError(t, err)
	require.False(t, user.Character.HasConditionFlag(conditions.Sleeping), "fixture: the sleep is refused")
	require.False(t, fired(), "a sleep refused in combat must not send the sleep command event")

	user.Character.EndAggro()
	require.False(t, user.Character.IsInCombat(), "fixture: the fight is over")
	fired = listenForCommand(t, user, 999242, "sleep")
	_, err = Sleep("", user, room, 0)
	require.NoError(t, err)
	require.True(t, user.Character.HasConditionFlag(conditions.Sleeping), "fixture: the player fell asleep")
	require.True(t, fired(), "a player who falls asleep must send the sleep command event")
}

func TestSearch_QuestCommandEventNotOnACooldownRefusal(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, _, room := speechWrapperScene(t)
	user.Character.Cooldowns = characters.Cooldowns{}
	t.Cleanup(func() { user.Character.Cooldowns = characters.Cooldowns{} })

	fired := listenForCommand(t, user, 999243, "search")
	_, err := Search("", user, room, 0)
	require.NoError(t, err)
	require.True(t, fired(), "a search that runs must send the search command event")

	require.False(t, user.Character.CooldownReady("search"), "fixture: the search spent its cooldown")
	fired = listenForCommand(t, user, 999244, "search")
	_, err = Search("", user, room, 0)
	require.NoError(t, err)
	require.False(t, fired(), "a search refused for cooldown must not send the search command event")
}

// craftQuestRecipe is a recipe with no station, skill floor or ingredients,
// so Craft() reaches the begin (or instant) branch in the test room.
func craftQuestRecipe(t *testing.T, id string, rounds int) *crafting.RecipeSpec {
	t.Helper()
	r := &crafting.RecipeSpec{
		RecipeId:       id,
		Name:           id,
		Skill:          "test-craft-quest-skill",
		TimeRounds:     rounds,
		Output:         crafting.RecipeOutput{ItemId: 10001, Quantity: 1},
		SuccessMessage: "You finish your work.",
	}
	crafting.RegisterRecipeForTest(r)
	t.Cleanup(func() { crafting.UnregisterRecipeForTest(id) })
	return r
}

// Beginning a timed craft must not count: the craft contest at the last round
// can still fail.
func TestCraft_QuestCommandEventNotWhenTheWorkBegins(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	recipe := craftQuestRecipe(t, "test-craft-quest-timed", 3)
	user.Character.KnownRecipes = map[string]int{recipe.RecipeId: 1}
	origActivity := user.Character.Activity
	user.Character.Activity = activity.NewMachine()
	t.Cleanup(func() {
		user.Character.KnownRecipes = nil
		user.Character.Activity = origActivity
	})

	fired := listenForCommand(t, user, 999245, "craft")
	_, err := Craft(recipe.Name, user, room, 0)
	require.NoError(t, err)
	require.True(t, user.Character.Activity.IsCrafting(), "fixture: the craft began")
	require.False(t, fired(), "a craft that has only begun must not send the craft command event")
}

// An instant recipe completes inside Craft() and cannot fail, so it counts
// at once. Both instant sites: Craft()'s ImmediateComplete branch and
// completeCraft (the enchanting instant path).
func TestCraft_QuestCommandEventWhenAnInstantCraftCompletes(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	recipe := craftQuestRecipe(t, "test-craft-quest-instant", 0)
	user.Character.KnownRecipes = map[string]int{recipe.RecipeId: 1}
	t.Cleanup(func() { user.Character.KnownRecipes = nil })

	fired := listenForCommand(t, user, 999246, "craft")
	_, err := Craft(recipe.Name, user, room, 0)
	require.NoError(t, err)
	require.True(t, fired(), "an instant craft that completes must send the craft command event")

	fired = listenForCommand(t, user, 999247, "craft")
	completeCraft(user, room, recipe)
	require.True(t, fired(), "completeCraft must send the craft command event")
}
