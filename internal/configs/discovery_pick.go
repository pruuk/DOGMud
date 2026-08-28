package configs

// WeightedDiscoveryPick chooses an index among discovery candidates, biased
// AWAY from harder ones.
//
// Why this exists, and why the bias is here rather than in DiscoveryChance.
// Discovery resolves in two steps: a chance is rolled, and only then is a
// candidate drawn from the eligible pool. At roll time there is no candidate,
// so there is no difficulty in scope -- putting a difficulty term into
// DiscoveryChance would mean inventing a value for a spell that has not been
// chosen yet. The honest place for "harder things surface less often" is the
// draw, which is where the difficulty actually exists.
//
// Weight per candidate is 1 / (1 + difficulty * DiscoveryDifficultyWeightScale),
// so at the shipped 0.02 a difficulty-50 candidate is drawn half as often as a
// difficulty-0 one that shares its pool. Nothing is excluded: the SKILL GATE
// decides what you can find at all (see spells.RequiredSkillFor and
// crafting.GetEligibleRecipes), and this only shades what you find first.
//
// difficulties must be parallel to the candidate slice the caller will index.
// Returns -1 for an empty pool. Falls back to a uniform draw when every weight
// is degenerate, so a bad config cannot wedge discovery.
func WeightedDiscoveryPick(difficulties []int, randN func(int) int) int {
	n := len(difficulties)
	if n == 0 {
		return -1
	}
	if n == 1 {
		return 0
	}

	scale := float64(GetBalanceConfig().DiscoveryDifficultyWeightScale)
	if scale <= 0 {
		return randN(n)
	}

	weights := make([]float64, n)
	total := 0.0
	for i, d := range difficulties {
		if d < 0 {
			d = 0
		}
		w := 1.0 / (1.0 + float64(d)*scale)
		weights[i] = w
		total += w
	}
	if total <= 0 {
		return randN(n)
	}

	// util.Rand takes an int, so the wheel is walked in integer units. 10000
	// units keeps the quantisation error far below the weight differences that
	// matter here (the smallest shipped gap, one difficulty point at scale
	// 0.02, is ~2% of a weight).
	const resolution = 10000
	target := float64(randN(resolution)) / float64(resolution) * total
	acc := 0.0
	for i, w := range weights {
		acc += w
		if target < acc {
			return i
		}
	}
	return n - 1
}
