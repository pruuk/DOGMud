package crafting

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// pinCraftBalanceForTest pins the knobs this file reasons about to their
// SHIPPED values.
//
// ⚠️ Load-bearing, not tidiness. A test binary never reads config.yaml, so
// SkillWeight arrives at its Go default of 2.0 while the game ships 5.0. Every
// number in the spec's mastery table is computed at 5.0, so without this pin
// the assertions below would be measuring a game nobody plays.
func pinCraftBalanceForTest(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.SkillWeight = 5.0
	cfg.Balance.CraftBaseDifficulty = 100
	cfg.Balance.CraftSkillMinWeight = 5
	configs.SetConfigForTest(t, cfg)
}

// TestCraftDifficultyAnchor pins the load-bearing property of spec 5.1.1: a
// baseline crafter (stat 100) holding EXACTLY the recipe's minimum skill, on
// neutral-tier materials, scores exactly the difficulty.
//
// Equal score and difficulty is a 50/50 contest, which reproduces today's
// shipped CraftingBaseSuccessChance of 50 with no special case. That is the
// entire reason CraftBaseDifficulty is 100 rather than an arbitrary number:
// 100 is the human stat baseline.
func TestCraftDifficultyAnchor(t *testing.T) {
	pinCraftBalanceForTest(t)

	for _, min := range []int{0, 5, 10, 40, 65} {
		score := CraftScore(100, min)
		diff := CraftDifficulty(min, 1.0)
		if math.Abs(score-diff) > 0.0001 {
			t.Errorf("skill_minimum %d: score %v != difficulty %v. A baseline "+
				"crafter at the recipe minimum must be exactly 50/50", min, score, diff)
		}
	}
}

// TestCraftMasteryCurveMatchesTheSpecTable pins spec 5.1.1's published table
// and, more importantly, the INTENT behind it: an advanced recipe takes far
// more mastery to become routine than a simple one.
//
// 🔴 This deliberately asserts that the two columns DIVERGE. An earlier draft of
// the plan asserted the opposite — that a fixed number of levels above the
// minimum should feel the same at both ends — which came from the SUPERSEDED
// formula in commit cfaf83958, where the per-level term was a percentage. The
// final design composes score as stat + skill*SkillWeight, and the spec is
// explicit: "A masterwork recipe needs roughly thirty levels above its minimum
// to become routine where a simple one needs nine. That is the intent."
//
// Probabilities are computed analytically rather than sampled, so this is an
// exact check with no flake surface. contest.Run draws both sides at a stdDev
// derived from the ATTACKER's score, so the difference of the two rolls is
// Normal with sd = sqrt(2) * RollSpread * score.
func TestCraftMasteryCurveMatchesTheSpecTable(t *testing.T) {
	pinCraftBalanceForTest(t)

	// success computes P(attackRoll > difficultyRoll) for the craft contest.
	success := func(score, diff float64) float64 {
		sd := math.Sqrt2 * 0.15 * score
		return 0.5 * (1 + math.Erf((score-diff)/sd/math.Sqrt2))
	}

	cases := []struct {
		levelsAbove      int
		wantSimple       float64 // skill_minimum 0
		wantMasterwork   float64 // skill_minimum 40
		tolerancePercent float64
	}{
		{0, 0.50, 0.50, 0.01},
		{5, 0.83, 0.64, 0.01},
		{9, 0.93, 0.73, 0.01},
		{20, 0.99, 0.88, 0.01},
		{30, 0.99, 0.94, 0.01},
	}

	for _, c := range cases {
		simple := success(CraftScore(100, c.levelsAbove), CraftDifficulty(0, 1.0))
		master := success(CraftScore(100, 40+c.levelsAbove), CraftDifficulty(40, 1.0))

		if math.Abs(simple-c.wantSimple) > c.tolerancePercent {
			t.Errorf("%d levels above a skill_minimum 0 recipe: %.1f%%, spec says %.0f%%",
				c.levelsAbove, simple*100, c.wantSimple*100)
		}
		if math.Abs(master-c.wantMasterwork) > c.tolerancePercent {
			t.Errorf("%d levels above a skill_minimum 40 recipe: %.1f%%, spec says %.0f%%",
				c.levelsAbove, master*100, c.wantMasterwork*100)
		}
		if c.levelsAbove > 0 && master >= simple {
			t.Errorf("%d levels above: masterwork %.1f%% must stay BELOW simple "+
				"%.1f%%. Equal columns mean SkillMinimum stopped driving "+
				"difficulty, which is the exact defect 5.1.1.1 records",
				c.levelsAbove, master*100, simple*100)
		}
	}
}

// TestMaterialTierMovesDifficulty pins that the material half is live. Before
// the PR #73 backfill every multiplier was the neutral 1.0 and this axis did
// nothing at all.
func TestMaterialTierMovesDifficulty(t *testing.T) {
	pinCraftBalanceForTest(t)

	common := CraftDifficulty(20, 0.75)
	neutral := CraftDifficulty(20, 1.0)
	rarest := CraftDifficulty(20, 1.25)

	if !(common < neutral && neutral < rarest) {
		t.Fatalf("material tier must raise difficulty monotonically: "+
			"%v / %v / %v", common, neutral, rarest)
	}
}

// TestDearestMaterialTierIsDeterministic pins spec 5.1.1.3 directly. Four items
// share component_tag "bottle", so a map-order resolver returns a different
// tier on different runs. Same inputs must give the same answer every time.
//
// Uses concrete ids (clay flask, glass vial, sealed phial, crystalline
// decanter) precisely because resolving them by TAG is the forbidden path.
func TestDearestMaterialTierIsDeterministic(t *testing.T) {
	consumed := []int{40043, 40006, 40044, 40045}

	first := DearestMaterialTier(consumed)
	for i := 0; i < 200; i++ {
		if got := DearestMaterialTier(consumed); got != first {
			t.Fatalf("tier flipped between identical calls: %v then %v. That is "+
				"the FindSpecByComponentTag map-order trap", first, got)
		}
	}
}

// TestDearestMaterialTierIsNeutralWhenUntiered pins that an unknown or untiered
// ingredient list yields 1.0 rather than the cheapest bucket, so a recipe whose
// materials are not yet tiered is not silently made EASIER.
func TestDearestMaterialTierIsNeutralWhenUntiered(t *testing.T) {
	if got := DearestMaterialTier(nil); got != 1.0 {
		t.Errorf("nil ingredient list = %v, want 1.0 (neutral)", got)
	}
	if got := DearestMaterialTier([]int{999999}); got != 1.0 {
		t.Errorf("unknown item = %v, want 1.0 (neutral)", got)
	}
}

// TestSalvageDifficultyReportsMissingRecipe pins the contract of the
// unreachable branch. Zero items carry salvage_returns today, so only crafted
// items are salvageable, but a caller that ignored ok would silently salvage
// everything at difficulty 0 — i.e. always succeed.
func TestSalvageDifficultyReportsMissingRecipe(t *testing.T) {
	if _, ok := SalvageDifficulty(999999, 1.0); ok {
		t.Fatal("an item with no recipe must report ok=false, not a difficulty")
	}
}
