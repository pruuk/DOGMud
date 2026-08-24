package hooks

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// charmMarginCeiling is the normalized margin at which a charm buys its full
// duration: two sigma.
//
// NOT the crit bar, despite the resemblance. combat.CritBarFor subtracts
// CritBarSkillSlope*(atkRank-defRank) and clamps to [CritBarFloor 1.5,
// CritBarCeiling 3.0], and because no mob in the game carries an authored
// rhetoric rank the real bar sits at 1.5 for any caster past manifestation 10.
// Calling this "the crit threshold" would be false for exactly the casters who
// use charm most, so it is named for what it is: two sigma.
const charmMarginCeiling = 2.0

// charmDurationFor converts the ATTACK-POSITIVE normalized margin of the
// winning contest into the number of rounds a charm holds.
//
//	duration = Min + (Max - Min) * clamp(margin / 2.0, 0, 1)
//
// The player is never told this number, nor how long is left. That uncertainty
// is the mechanic (spec 3.3): a bond you cannot plan around is the whole risk
// of charming something dangerous.
//
// A FLOORED win arrives here as margin 0 and therefore takes Min, which is
// correct rather than incidental -- a mercy-granted success is not a dominant
// one and must not read as dominance.
//
// A FORCED crit (a sleeping victim) also arrives as 0, because the seam returns
// above its margin assignment. That one is NOT correct, and the caller corrects
// it before calling here; see spec 15 and applyMobEffect_charm. This function
// deliberately does not know about crits -- it maps a margin, nothing else.
func charmDurationFor(normalizedMargin float64) int {
	bal := configs.GetBalanceConfig()

	lo := int(bal.CharmDurationMinRounds)
	hi := int(bal.CharmDurationMaxRounds)
	if lo < 1 {
		lo = 1
	}
	// Guard independently of the config validator: an inverted pair must
	// collapse to a fixed duration rather than run the curve backwards, and
	// this may be called against a config that never went through validation.
	if hi < lo {
		hi = lo
	}

	ratio := normalizedMargin / charmMarginCeiling
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return lo + int(math.Round(float64(hi-lo)*ratio))
}
