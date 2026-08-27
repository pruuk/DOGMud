package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

// U10b-1 Task 16: a FAILED craft awards ProgressionFailureFraction.
//
// ⚠️ THIS IS THE CASE THE WHOLE SLICE IS JUSTIFIED BY. Crafting was
// success-only: a crafter who burned their materials on a botched attempt
// learned literally nothing from it, which is the single most obviously wrong
// consequence of the old firing convention. If this does not hold, nothing else
// in U10b-1 matters.
//
// Driven through tickMobCrafting rather than the player round tick because it
// is a package-level function taking only a mob -- the player path is the same
// code buried eight levels deep inside NewRound_UserRoundTick and is not
// reachable without standing up a user session.
//
// The craft chance is pinned by config rather than by rank arithmetic:
// CalcSuccessChance clamps to [CraftingMinSuccessChance, CraftingMaxSuccessChance],
// so setting both ends to the same value makes the outcome exact.
func craftProgressionMob(t *testing.T, recipeId, skill string) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{InstanceId: 7701}
	m.Character.Name = "CraftFailer"
	m.Character.Activity = activity.NewMachine()
	m.Character.Skills = map[string]int{skill: 1}
	_ = m.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: recipeId, RoundsTotal: 1},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	return m
}

// pinCraftChance forces every craft roll to the same verdict by collapsing the
// clamp range onto one value.
func pinCraftChance(t *testing.T, pct int) {
	t.Helper()
	pinConfigForTest(t)
	cfg := configs.GetConfig()
	cfg.Balance.CraftingMinSuccessChance = configs.ConfigInt(pct)
	cfg.Balance.CraftingMaxSuccessChance = configs.ConfigInt(pct)
	configs.SetConfigForTest(t, cfg)
}

func TestTickMobCrafting_AFailedCraftStillAwards(t *testing.T) {
	const recipeId, skill = "u10b1-craft-fail", "blacksmithing"

	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{
		RecipeId: recipeId, Skill: skill, SkillMinimum: 0,
	})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest(recipeId) })

	pinCraftChance(t, 0) // util.Rand(100) is never < 0, so the craft ALWAYS fails

	mob := craftProgressionMob(t, recipeId, skill)
	before := mob.Character.GetSkillUseCount(skill)

	tickMobCrafting(mob)

	if got := mob.Character.GetSkillUseCount(skill) - before; got != 1 {
		t.Fatalf("%s use count rose by %d after a FAILED craft, want 1; a botched craft must still train at the failure fraction", skill, got)
	}
}

// The mirror: a SUCCEEDING craft awards too, and exactly once. Without this,
// the test above would pass against an implementation that awarded on failure
// only, or one that awarded twice.
func TestTickMobCrafting_ASucceedingCraftAwardsOnce(t *testing.T) {
	const recipeId, skill = "u10b1-craft-win", "blacksmithing"

	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{
		RecipeId: recipeId, Skill: skill, SkillMinimum: 0,
	})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest(recipeId) })

	pinCraftChance(t, 100) // util.Rand(100) is always < 100, so it ALWAYS succeeds

	mob := craftProgressionMob(t, recipeId, skill)
	before := mob.Character.GetSkillUseCount(skill)

	tickMobCrafting(mob)

	if got := mob.Character.GetSkillUseCount(skill) - before; got != 1 {
		t.Fatalf("%s use count rose by %d after a successful craft, want exactly 1", skill, got)
	}
}
