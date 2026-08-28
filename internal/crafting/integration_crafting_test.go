package crafting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// ─── Integration: Full Crafting Loop ─────────────────────────────────────────
// Exercises: seedRegistry, HasIngredients(), ConsumeIngredients(),
// CraftScore()/CraftDifficulty(), GetStarterRecipes(), GetEligibleRecipes()

func TestIntegration_CraftingFullLoop(t *testing.T) {
	seedRegistry()

	recipe := GetRecipe("iron-dagger")
	assert.NotNil(t, recipe, "iron-dagger recipe should exist")

	// Simulate a character with blacksmithing skill and ingredients
	skillLevel := 5
	inv := []items.Item{
		makeItem("iron-ingot"),
		makeItem("leather-strip"),
		makeItem("other-junk"),
	}

	// Step 1: Check ingredients
	ok, missing := HasIngredients(inv, []items.Item{}, recipe)
	assert.True(t, ok, "Should have all ingredients")
	assert.Empty(t, missing, "No missing ingredients")

	// Step 2: the craft CONTEST (U10b-1b replaced the flat percentage).
	// A crafter ABOVE the recipe minimum out-scores its difficulty.
	score := CraftScore(100, skillLevel)
	difficulty := CraftDifficulty(recipe.SkillMinimum, 1.0)
	assert.Greater(t, score, difficulty,
		"a crafter above the recipe minimum must out-score its difficulty")

	// Step 3: Consume ingredients
	remaining, _ := ConsumeIngredients(inv, []items.Item{}, recipe)
	assert.Len(t, remaining, 1, "Only the junk item should remain")
	assert.Equal(t, "other-junk", remaining[0].GetSpec().ComponentTag,
		"Remaining item should be the non-ingredient")

	// Step 4: Verify ingredients are gone
	ok, _ = HasIngredients(remaining, []items.Item{}, recipe)
	assert.False(t, ok, "Should no longer have ingredients after consuming")
}

func TestIntegration_CraftingInsufficientSkill(t *testing.T) {
	seedRegistry()

	recipe := GetRecipe("iron-buckler") // SkillMinimum = 5

	// Character with low skill
	skillLevel := 1

	// A crafter BELOW the minimum is out-scored by the difficulty.
	score := CraftScore(100, skillLevel)
	difficulty := CraftDifficulty(recipe.SkillMinimum, 1.0)
	assert.Less(t, score, difficulty,
		"a crafter below the recipe minimum must be out-scored by its difficulty")

	// The old 5%% floor is now CraftFloor, applied inside RunCraftContest as a
	// symmetric mercy band rather than a clamp on a percentage.
	assert.Greater(t, float64(configs.GetBalanceConfig().CraftFloor), 0.0,
		"the mercy band must exist; a 0 floor would delete it")
}

func TestIntegration_CraftingIngredientsNotConsumedOnCheck(t *testing.T) {
	seedRegistry()

	recipe := GetRecipe("iron-dagger")
	inv := []items.Item{
		makeItem("iron-ingot"),
		makeItem("leather-strip"),
	}

	// HasIngredients should NOT modify the inventory
	ok, _ := HasIngredients(inv, []items.Item{}, recipe)
	assert.True(t, ok)
	assert.Len(t, inv, 2, "HasIngredients should not modify inventory")

	// Call it again — should still pass
	ok, _ = HasIngredients(inv, []items.Item{}, recipe)
	assert.True(t, ok, "Repeated HasIngredients should still pass")
}

func TestIntegration_CraftingMultiQuantityIngredient(t *testing.T) {
	seedRegistry()

	buckler := GetRecipe("iron-buckler") // needs 2x iron-ingot + 1x wooden-plank

	// Only 1 iron-ingot — should fail
	inv := []items.Item{
		makeItem("iron-ingot"),
		makeItem("wooden-plank"),
	}
	ok, missing := HasIngredients(inv, []items.Item{}, buckler)
	assert.False(t, ok, "Should fail with only 1 iron-ingot")
	assert.Equal(t, "iron-ingot", missing, "Missing ingredient should be iron-ingot")

	// Add second iron-ingot — should pass
	inv = append(inv, makeItem("iron-ingot"))
	ok, _ = HasIngredients(inv, []items.Item{}, buckler)
	assert.True(t, ok, "Should pass with 2 iron-ingots")

	// Consume and verify
	remaining, _ := ConsumeIngredients(inv, []items.Item{}, buckler)
	assert.Len(t, remaining, 0, "All items should be consumed for buckler")
}

func TestIntegration_RecipeDiscovery(t *testing.T) {
	seedRegistry()

	// Starter recipes: SkillMinimum == 0
	starters := GetStarterRecipes()
	assert.Contains(t, starters, "iron-dagger",
		"iron-dagger (min=0) should be a starter")
	assert.Contains(t, starters, "healing-salve",
		"healing-salve (min=0) should be a starter")
	assert.NotContains(t, starters, "healing-poultice",
		"healing-poultice (min=1) should NOT be a starter")
	assert.NotContains(t, starters, "iron-buckler",
		"iron-buckler (min=5) should NOT be a starter")

	// Simulate a character who knows starters and has blacksmithing=5
	knownRecipes := map[string]int{
		"iron-dagger":      1,
		"healing-poultice": 1,
	}
	skills := map[string]int{
		"blacksmithing": 5,
		"alchemy":       0,
	}

	eligible := GetEligibleRecipes(knownRecipes, skills, "blacksmithing")

	// iron-buckler (min=5, blacksmithing) should now be discoverable
	found := false
	for _, id := range eligible {
		if id == "iron-buckler" {
			found = true
		}
		// Known recipes should never appear
		assert.NotEqual(t, "iron-dagger", id,
			"Already known recipes should not be eligible")
		assert.NotEqual(t, "healing-poultice", id,
			"Already known recipes should not be eligible")
	}
	assert.True(t, found, "iron-buckler should be eligible at skill 5")

	// If skill is too low, nothing should be eligible
	skills["blacksmithing"] = 3
	eligible = GetEligibleRecipes(knownRecipes, skills, "blacksmithing")
	for _, id := range eligible {
		assert.NotEqual(t, "iron-buckler", id,
			"iron-buckler should not be eligible with skill 3")
	}
}

// TestIntegration_CraftScoreTracksSkillAgainstTheMinimum replaces the old
// success-chance range table. U10b-1b retired the flat percentage, so the
// meaningful assertion is no longer "skill 6 vs min 5 gives 55%" but the
// ORDERING: at the minimum a crafter ties the difficulty, above it they lead,
// below it they trail.
//
// A tie is the anchor that matters — it is what makes a baseline crafter at the
// recipe minimum exactly 50/50, reproducing the old CraftingBaseSuccessChance
// with no special case.
func TestIntegration_CraftScoreTracksSkillAgainstTheMinimum(t *testing.T) {
	pinCraftBalanceForTest(t) // SkillWeight 5.0; a test binary defaults it to 2.0

	const stat = 100.0

	tests := []struct {
		name  string
		skill int
		min   int
		cmp   string // "tie", "above", "below"
	}{
		{"at minimum", 5, 5, "tie"},
		{"1 above", 6, 5, "above"},
		{"far above", 20, 5, "above"},
		{"below minimum", 3, 5, "below"},
		{"far below", 0, 5, "below"},
		{"zero skill zero min", 0, 0, "tie"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CraftScore(stat, tt.skill)
			difficulty := CraftDifficulty(tt.min, 1.0)
			switch tt.cmp {
			case "tie":
				assert.InDelta(t, difficulty, score, 0.0001,
					"a crafter exactly at the minimum must tie the difficulty (50/50)")
			case "above":
				assert.Greater(t, score, difficulty)
			case "below":
				assert.Less(t, score, difficulty)
			}
		})
	}
}

func TestIntegration_RecipeRegistry(t *testing.T) {
	seedRegistry()

	// GetAll should return all seeded recipes
	all := GetAll()
	assert.Len(t, all, 4, "Should have 4 seeded recipes")

	// GetRecipe for each
	assert.NotNil(t, GetRecipe("iron-dagger"))
	assert.NotNil(t, GetRecipe("iron-buckler"))
	assert.NotNil(t, GetRecipe("healing-poultice"))
	assert.NotNil(t, GetRecipe("healing-salve"))
	assert.Nil(t, GetRecipe("nonexistent"))

	// FindRecipeByName
	r := FindRecipeByName("poultice")
	assert.NotNil(t, r)
	assert.Equal(t, "healing-poultice", r.RecipeId)

	// GetAllForSkill
	blacksmithing := GetAllForSkill("blacksmithing")
	assert.Len(t, blacksmithing, 2, "Should have 2 blacksmithing recipes")

	alchemy := GetAllForSkill("alchemy")
	assert.Len(t, alchemy, 2, "Should have 2 alchemy recipes")
}
