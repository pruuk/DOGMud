package combat

import "math"

// Chunk 5.11d — crit derives from the normalized margin of the opposed roll.
//
// Before this, crit was keyed on the attacker's SELF-RELATIVE z-score
// (zScore = (value - mean) / stdDev), which is statistically independent of the
// opponent: a stat-200 attacker facing a stat-50 defender critted at the same
// base rate as an even match. Winning decisively could not make you crit.
//
// That is why calcCritThreshold existed in the shape it did — it was the only
// channel through which anything about the matchup could reach crit, so skill,
// Accuracy, Blink and grapple were all hand-injected into one scalar with about
// 1.0 of headroom, where they collided and saturated. Deriving crit from the
// margin instead means skill, stats, gear and position all raise crit rate
// automatically, because they all raise the margin.

// normalizedAttackMargin converts an opposed-roll outcome into the ATTACKER's
// margin expressed in standard deviations, suitable for comparison against a
// z-style crit threshold.
//
// The second return is false when there is no usable margin, in which case the
// caller must fall back to the legacy self-relative z-score. Callers must not
// treat a false as "no crit".
//
// Three traps are handled here, all of them silent if got wrong:
//
// T1 — SIGN. runBestOfAllDefense computes
// `margin := defenseRoll.Value - attackRoll.Value` and keeps the LARGEST, so
// best.margin is DEFENCE-positive: a positive value means the defence won. The
// attacker's margin is therefore its negation. Getting this backwards puts crit
// on the losing side and compiles cleanly.
//
// T2 — INFINITY. best.margin is initialised to math.Inf(-1) and only overwritten
// inside the defence loop. A defender with no stamina, or an empty defence
// sequence, leaves it there. Negated that is +Inf, which under margin-derivation
// reads as an infinitely decisive attack and would crit EVERY swing. It must be
// detected via defenseType — never by testing the margin value, which is exactly
// the check that looks reasonable and fails.
//
// T3 — NORMALISER. Both the attack roll and every defence roll are made with the
// attacker's stdDev (`atkStdDev := dice.StdDevFor(atkScore)`), so their
// difference has standard deviation stdDev*sqrt(2). Dividing by stdDev alone
// inflates the result by about 41% and silently raises crit rates everywhere.
func normalizedAttackMargin(best bestDefenseResult) (float64, bool) {
	// T2: no defence attempted means no contest, and no margin to derive from.
	if best.defenseType == "" {
		return 0, false
	}

	stdDev := best.hitRoll.StdDev
	if stdDev <= 0 {
		return 0, false
	}

	// T1: negate to convert defence-positive into attacker-positive.
	// T3: sqrt(2) because both sides rolled with the same stdDev.
	return -best.margin / (stdDev * math.Sqrt2), true
}

// normalizedDefenseMargin is the mirror of normalizedAttackMargin, signed from
// the DEFENDER's perspective. best.margin is already defence-positive, so no
// negation is needed here — which is precisely why the attacker side does need
// one.
func normalizedDefenseMargin(best bestDefenseResult) (float64, bool) {
	if best.defenseType == "" {
		return 0, false
	}

	stdDev := best.defRoll.StdDev
	if stdDev <= 0 {
		return 0, false
	}

	return best.margin / (stdDev * math.Sqrt2), true
}
