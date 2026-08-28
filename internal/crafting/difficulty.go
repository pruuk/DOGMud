package crafting

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// CraftScore composes the crafter's side the way EVERY score in this game is
// composed: stat + skill * SkillWeight.
//
// 🔴 Do NOT invent a bespoke formula here. Spec 5.1.1.1 records that an earlier
// draft composed the score as
// (CraftBaseDifficulty + SkillMinimum*CraftSkillMinWeight) * (1 + levels*0.05),
// putting the recipe's anchor on BOTH sides. It cancelled exactly, which made
// CraftSkillMinWeight a knob that could never change an outcome, removed
// SkillMinimum from the odds entirely, and ignored the crafter's stat
// altogether.
//
// `stat` is the PRIMARY STAT OF THE DISCIPLINE and differs per craft:
// blacksmithing reads strength, alchemy/cooking/enchanting perception,
// tailoring/jewelcrafting dexterity (skills.SkillPrimaryStats). Callers resolve
// it; this function must not guess, because guessing wrong is silent.
func CraftScore(stat float64, skillLevel int) float64 {
	w := float64(configs.GetBalanceConfig().SkillWeight)
	return stat + float64(skillLevel)*w
}

// CraftDifficulty is the recipe's side of the contest.
//
// materialTierMult comes from items.MaterialTierMultiplier over the DEAREST
// ingredient ACTUALLY BEING CONSUMED — see DearestMaterialTier, and spec
// 5.1.1.3 for why it must never be resolved through a component_tag.
//
// CraftBaseDifficulty is 100 because 100 is the human stat baseline, so a
// baseline crafter holding exactly the recipe's minimum scores 100 + min*5
// against 100 + min*5 and is a coin flip. That reproduces the shipped
// CraftingBaseSuccessChance of 50 with no special case.
func CraftDifficulty(skillMinimum int, materialTierMult float64) float64 {
	b := configs.GetBalanceConfig()
	base := float64(b.CraftBaseDifficulty)
	perMin := float64(b.CraftSkillMinWeight)
	return (base + float64(skillMinimum)*perMin) * materialTierMult
}

// DearestMaterialTier returns the multiplier for the highest MaterialTier among
// the items that will actually be consumed.
//
// 🔴 TAKES THE ITEMS THEMSELVES, not ids, and reads each item's OWN spec. An
// earlier draft took []int and looked each id up in the global registry, which
// was wrong twice: it re-derives data SelectIngredients already has, and a
// registry lookup answers for the TEMPLATE rather than for the instance the
// player is actually spending.
//
// 🔴 NEVER RESOLVE BY component_tag. Spec 5.1.1.3:
// items.FindSpecByComponentTag iterates a Go map, and FOUR items share
// component_tag "bottle" (Clay Flask 1, Glass Vial 1, Sealed Phial 3,
// Crystalline Decanter 4). Resolving a recipe's tag through it would re-roll
// the tier on every attempt, swinging an alchemy craft's odds with no cause a
// player could observe.
//
// An empty list, or items that are all untiered, yields the neutral 1.0 —
// MaterialTierMultiplier(0) is 1.0, not the cheapest bucket, so partial
// backfill coverage cannot silently make a recipe easy.
func DearestMaterialTier(consumed []items.Item) float64 {
	best := 0
	for _, it := range consumed {
		// GetSpec returns a VALUE, not a pointer, so there is nothing to
		// nil-check: an unknown item yields the zero spec, whose MaterialTier
		// is 0, which is exactly the neutral path we want for it.
		if tier := it.GetSpec().MaterialTier; tier > best {
			best = tier
		}
	}
	return items.MaterialTierMultiplier(best)
}

// SalvageDifficulty is the CRAFT difficulty of the item being taken apart: as
// hard to unmake as it was to make (owner, 2026-08-26).
//
// Reading the recipe of the item BEING CONSUMED sidesteps a trap an earlier
// draft could not: it needed the tier of a material that does not exist yet
// (salvage is what CREATES it), and the only tag resolver is the map-order one
// forbidden by 5.1.1.3. GetRecipeByOutputItemId is already indexed.
//
// Returns ok=false when the item has no recipe. UNREACHABLE TODAY — zero items
// in _datafiles/world/dogmud/items/ carry salvage_returns, so only crafted
// items are salvageable — but callers must still handle it.
func SalvageDifficulty(itemId int, materialTierMult float64) (float64, bool) {
	r := GetRecipeByOutputItemId(itemId)
	if r == nil {
		return 0, false
	}
	return CraftDifficulty(r.SkillMinimum, materialTierMult), true
}

// RunCraftContest is THE entry point for every craft resolution, and the ONE
// place Balance.CraftFloor is read.
//
// 🔴 DO NOT CALL contest.RunWithFloors DIRECTLY FROM A CRAFT SITE. This mirrors
// combat.RunContest, whose doc comment records exactly why: before U6 there
// were three wrapper pairs over eight floor knobs, and because config.yaml
// shipped them at similar values, wiring a site to the WRONG pair was invisible
// in production and became a live balance bug the moment one pair was retuned.
//
// CraftFloor 0.05 and SalvageFloor 0.15 are close enough that passing the wrong
// one would show up in no test, and would surface only as "crafting feels off"
// after a retune. One named reader per floor removes the failure mode instead
// of guarding against it. Craft sites must not be able to name a floor at all.
func RunCraftContest(score, difficulty float64) contest.Result {
	return contest.RunWithFloors(score, []contest.Entry{{Score: difficulty}},
		float64(configs.GetBalanceConfig().CraftFloor))
}

// RunSalvageContest is THE entry point for every salvage resolution, and the
// ONE place Balance.SalvageFloor is read. See RunCraftContest for why this is a
// named seam rather than an inline call.
func RunSalvageContest(score, difficulty float64) contest.Result {
	return contest.RunWithFloors(score, []contest.Entry{{Score: difficulty}},
		float64(configs.GetBalanceConfig().SalvageFloor))
}
