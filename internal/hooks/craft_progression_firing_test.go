package hooks

import (
	"github.com/stretchr/testify/require"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
// The craft verdict is pinned by pinCraftChance; see its doc for why that had
// to change with U10b-1b.
func craftProgressionMob(t *testing.T, recipeId, skill string) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{InstanceId: 7701}
	m.Character.Name = "CraftFailer"
	m.Character.Activity = activity.NewMachine()
	m.Character.Skills = map[string]int{skill: 1}
	// Craft is a CONTEST since U10b-1b, and a bare Mob has no stats. At score 0
	// dice.StdDevFor(0) is 0, so the attacker rolls a deterministic 0 and can
	// never win no matter how low the difficulty is set.
	m.Character.Stats.Strength.Base = 500
	m.Character.Stats.Perception.Base = 500
	m.Character.Stats.Dexterity.Base = 500
	m.Character.Stats.Strength.Recalculate()
	m.Character.Stats.Perception.Recalculate()
	m.Character.Stats.Dexterity.Recalculate()
	_ = m.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: recipeId, RoundsTotal: 1},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	return m
}

// pinCraftChance forces every craft to the same verdict: success for pct >= 100,
// failure otherwise.
//
// 🔴 IT NO LONGER PINS CraftingMin/MaxSuccessChance. U10b-1b retired
// CalcSuccessChance as the craft decision, so those knobs decide nothing and a
// test setting them was asserting against an unpinned contest. Measured before
// this fix: pinCraftChance(t, 100) produced a 4.95% success rate — the
// CraftFloor and nothing else — so the "succeeding craft" test was running the
// FAILURE branch about 95% of the time and passing only because the award fires
// above the branch. It had stopped disambiguating itself from its own mirror,
// which is the single thing it was written to do.
//
// Pins the CONTEST instead: collapse or inflate the difficulty, and suppress the
// mercy floor, which otherwise flips outcomes in both directions.
func pinCraftChance(t *testing.T, pct int) {
	t.Helper()
	pinConfigForTest(t)
	cfg := configs.GetConfig()
	if pct >= 100 {
		cfg.Balance.CraftBaseDifficulty = 1
	} else {
		cfg.Balance.CraftBaseDifficulty = 1000000
	}
	cfg.Balance.CraftSkillMinWeight = 0
	cfg.Balance.CraftFloor = configs.ConfigFloat(1e-12)
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

// U10b-1 Task 20: first-kill progression is deleted and must stay deleted.
//
// OnFirstMobKill fired a bonus progression check on the first kill of each mob
// type, at CritProgressionBonus weight. It went for two reasons:
//
//  1. It is not a resolved action. Killing a mob type you have not killed
//     before is a milestone, not a contest, so it has no place in a firing
//     convention built on win/lose.
//  2. ⚠️ It progressed a skill literally named "combat", which DOES NOT EXIST.
//     There is no Combat SkillTag, "combat" is absent from
//     skills.SkillPrimaryStats, and skillNameMap is empty so nothing aliased
//     it. Verified against the archived prod saves: none of the 34 carrying a
//     skills block has a `combat:` entry, so the phantom skill never reached
//     player data and no save cleanup is owed.
//
// KD.AddMobKill is deliberately KEPT -- it is the kill-count statistic, not
// progression, and the leaderboard reads it.
func TestFirstMobKillProgression_StaysDeleted(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller(0) failed")
	internalDir := filepath.Dir(filepath.Dir(thisFile))

	var offenders []string
	err := filepath.WalkDir(internalDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(src), "OnFirstMobKill") {
			offenders = append(offenders, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, offenders, "OnFirstMobKill is back; it progressed a skill named \"combat\" that does not exist")

	// The kill-count statistic must survive the deletion.
	credit, err := os.ReadFile(filepath.Join(internalDir, "hooks", "Death_MobKillCredit.go"))
	require.NoError(t, err)
	require.Contains(t, string(credit), "KD.AddMobKill(",
		"KD.AddMobKill was deleted along with the progression call; it is the kill-count stat, not progression")
}
