package conversation

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// RecipeSkillMet is the craft command's own test: the recipe's skill at or
// above its minimum, and false for a recipe nobody shipped.
func TestPlayerState_RecipeSkillMetMatchesTheCraftTest(t *testing.T) {
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: "zz-ps-recipe", Skill: "zzps", SkillMinimum: 12})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest("zz-ps-recipe") })

	u := users.NewTestUser(7102, "recipe", "Recipia", 7102)
	ps := PlayerState(u)
	u.Character.Skills = map[string]int{"zzps": 11}
	require.False(t, ps.RecipeSkillMet("zz-ps-recipe"), "one below the minimum")
	u.Character.Skills["zzps"] = 12
	require.True(t, ps.RecipeSkillMet("zz-ps-recipe"), "at the minimum")
	require.False(t, ps.RecipeSkillMet("zz-no-such-recipe"), "unknown recipe fails closed")
}

func TestPlayerState_KnowsAndLearnsRecipes(t *testing.T) {
	u := users.NewTestUser(7103, "learn", "Learnia", 7103)
	ps := PlayerState(u)
	require.False(t, ps.KnowsRecipe("zz-learned"))
	ps.LearnRecipe("zz-learned")
	require.True(t, u.Character.HasRecipe("zz-learned"))
	require.True(t, ps.KnowsRecipe("zz-learned"))
}
