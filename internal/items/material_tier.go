package items

import "github.com/GoMudEngine/GoMud/internal/configs"

// MaterialTierMultiplier maps an authored material tier onto the craft
// difficulty band. Five buckets, evenly spaced, tier 3 neutral:
//
//	1 -> 0.95   common
//	2 -> 0.975
//	3 -> 1.0    neutral
//	4 -> 1.025
//	5 -> 1.05   rarest
//
// WHY THIS EXISTS. Crafting had no difficulty model at all: a flat percentage
// of skill against recipe.SkillMinimum, with nothing to contest against.
// U10b-1b gave craft a real difficulty, and the material tier is the half of it
// that comes from WHAT YOU ARE WORKING WITH rather than which recipe you picked.
//
// ✅ READ BY crafting.DearestMaterialTier as of U10b-1b Phase B. It shipped
// ahead of that consumer so the 138-file backfill could proceed independently,
// and it had to ship first, because fileloader's strict-decode probe (installed
// by boot_smoke_test.go) fails any YAML key that maps to no Go field.
//
// 🔴 AN ABSENT TIER IS NEUTRAL, NOT CHEAP. Tier 0 returns 1.0, not the Min. If an
// untiered material returned the cheapest multiplier, every file still awaiting
// the backfill would be quietly making its recipes EASIER, and completing the
// backfill would read as a difficulty increase rather than as the model coming
// online. Partial coverage must be inert. A negative tier is nonsense rather
// than a bucket, so it takes the same neutral path.
//
// Out-of-range positive tiers clamp to the top bucket. The authoring guard
// rejects new materials with no tier, but nothing stops a typo writing 7, and
// the item loader is lenient by design in production.
func MaterialTierMultiplier(tier int) float64 {
	if tier <= 0 {
		return 1.0
	}

	b := configs.GetBalanceConfig()
	lo := float64(b.MaterialTierMultiplierMin)
	hi := float64(b.MaterialTierMultiplierMax)

	const buckets = 5
	if tier > buckets {
		tier = buckets
	}

	// Evenly spaced across the band: bucket 1 sits at lo, bucket 5 at hi.
	return lo + (hi-lo)*(float64(tier-1)/float64(buckets-1))
}
