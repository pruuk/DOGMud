package crafting

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// CalcSalvageChance returns the per-ingredient recovery probability for
// a given salvage skill level. Uses a sqrt curve:
//
//	chance = min + (max - min) * sqrt(clamp(skill, 1, softCap) / softCap)
func CalcSalvageChance(skill int, minChance, maxChance float64, softCap int) float64 {
	if skill < 1 {
		skill = 1
	}
	ratio := float64(skill) / float64(softCap)
	if ratio > 1.0 {
		ratio = 1.0
	}
	return minChance + (maxChance-minChance)*math.Sqrt(ratio)
}

// CalcSalvageRounds determines how many rounds a salvage attempt takes
// based on the total gold value of ingredients.
func CalcSalvageRounds(totalGoldValue int, goldPerRound int, maxRounds int) int {
	if goldPerRound < 1 {
		goldPerRound = 10
	}
	rounds := totalGoldValue / goldPerRound
	if rounds < 1 {
		rounds = 1
	}
	if rounds > maxRounds {
		rounds = maxRounds
	}
	return rounds
}

// RollSalvageReturns contests each unit of each ingredient independently and
// returns the items recovered.
//
// U10b-1b: every unit is a salvage CONTEST of score against difficulty, where
// the difficulty is the item's own CRAFT difficulty -- as hard to unmake as it
// was to make (owner, 2026-08-26). It replaces a flat per-unit percentage off a
// sqrt curve.
//
// 🔴 The curve is REPLACED, not approximated. Spec 5.1.1.4 records that a blind
// review proved NO single difficulty reproduces it: matching today's rate needs
// 123 at skill 0, ~172 at skill 15 and ~199 at skill 25, because a contest is a
// normal CDF of a ratio and saturates earlier than 0.15 + 0.70*sqrt(s/50). The
// shapes cannot be reconciled, so deriving difficulty from the recipe stops
// trying.
func RollSalvageReturns(ingredients []RecipeIngredient, score, difficulty float64) []RecipeIngredient {
	var recovered []RecipeIngredient
	for _, ing := range ingredients {
		qty := 0
		for i := 0; i < ing.Quantity; i++ {
			if RunSalvageContest(score, difficulty).Success {
				qty++
			}
		}
		if qty > 0 {
			recovered = append(recovered, RecipeIngredient{
				ItemTag:  ing.ItemTag,
				Quantity: qty,
			})
		}
	}
	return recovered
}

// RollSalvageReturnsFromSpec contests salvage returns for tagged items (items
// with SalvageReturns on their ItemSpec).
//
// ⚠️ UNREACHABLE TODAY: zero items in _datafiles/world/dogmud/items/ carry
// salvage_returns, so only crafted items are salvageable. Converted for
// consistency, deliberately not tuned.
func RollSalvageReturnsFromSpec(returns []items.SalvageReturn, score, difficulty float64) []RecipeIngredient {
	var recovered []RecipeIngredient
	for _, ret := range returns {
		qty := 0
		for i := 0; i < ret.Quantity; i++ {
			if RunSalvageContest(score, difficulty).Success {
				qty++
			}
		}
		if qty > 0 {
			recovered = append(recovered, RecipeIngredient{
				ItemTag:  ret.ItemTag,
				Quantity: qty,
			})
		}
	}
	return recovered
}

// CalcIngredientGoldValue sums the gold value of all ingredients in a recipe
// by looking up each component tag's item value.
func CalcIngredientGoldValue(ingredients []RecipeIngredient) int {
	total := 0
	for _, ing := range ingredients {
		if spec := items.FindSpecByComponentTag(ing.ItemTag); spec != nil {
			total += spec.Value * ing.Quantity
		}
	}
	return total
}

// CalcSalvageReturnGoldValue sums the gold value of salvage returns from
// tagged items.
func CalcSalvageReturnGoldValue(returns []items.SalvageReturn) int {
	total := 0
	for _, ret := range returns {
		if spec := items.FindSpecByComponentTag(ret.ItemTag); spec != nil {
			total += spec.Value * ret.Quantity
		}
	}
	return total
}
