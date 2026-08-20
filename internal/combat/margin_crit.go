package combat

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/dice"
)

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
// T1 — SIGN. Since U1, the margin itself is computed inside contest.Run, not
// here: it rolls the attack once, contests every defence against it, and keeps
// the SMALLEST `attackRoll.Value - defenseRoll.Value` — an ATTACK-positive
// convention, the opposite of this package's. runBestOfAllDefense negates that
// result at the boundary (`best.margin = -res.Margin`) to preserve
// bestDefenseResult's DEFENCE-positive convention: a positive best.margin means
// the defence won. The attacker's margin here is therefore its negation.
// Getting this backwards puts crit on the losing side and compiles cleanly.
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

// ContestCritThreshold is the margin, in standard deviations, at or above which
// an opposed roll counts as a critical success. 2.0 reproduces the legacy
// self-relative z >= 2.0 rate (~2.3%) at parity by construction, so evenly
// matched contests are unchanged by the switch to margin derivation.
const ContestCritThreshold = 2.0

// ContestCrit reports whether one side of a contest won decisively enough to
// crit. It is the spell/conviction-channel counterpart to the melee path in
// resolveDefenseOutcome, and exists so those channels stop deriving crit from a
// self-relative z-score that knows nothing about the opponent.
//
// `margin` MUST be a contest.Result.Margin, and MUST already be signed from the
// perspective of the side being tested. Result.Margin is ATTACK-positive
// (attack roll minus winning defence roll), so:
//
//	attacker's crit check  -> pass it unnegated  (combat_taunt.go, the spell
//	                          sites in internal/hooks)
//	defender's crit check  -> negate it          (ResolveChannelDefence)
//
// NEVER pass a dice.RollResult's .Margin field. Since U1-U3 every caller
// resolves through internal/contest, which rolls each side with dice.Roll, and
// dice.Roll does NOT populate RollResult.Margin. Reading res.AttackRoll.Margin
// or res.DefenseRoll.Margin therefore hands this function a silent zero, and
// nothing crits again -- no compile error, no test failure, no log line.
//
// Do NOT copy the negation in normalizedAttackMargin. That path reads
// bestDefenseResult.margin, which runBestOfAllDefense builds DEFENCE-positive;
// these two conventions are opposites and mixing them compiles cleanly while
// putting crit on the losing side.
//
// `roll` supplies the normaliser and the legacy fallback. Both sides of an
// opposed roll are made with the attacker's stdDev, so their difference has
// standard deviation stdDev*sqrt(2) — dividing by stdDev alone inflates the
// result by about 41% and would roughly triple crit rates.
//
// Note on contest floors: contest.RunWithFloors overwrites the margin with +-1
// when a floor fires. That sentinel normalises to a near-zero z, so a hit handed
// out by the floor cannot also be a crit. This is intended.
func ContestCrit(margin float64, roll dice.RollResult) bool {
	return ContestCritAt(margin, roll, ContestCritThreshold)
}

// ContestCritAt is ContestCrit against a caller-supplied bar instead of the
// constant threshold — the channel seam passes CritBarFor's clamped skill-pair
// bar (U6b Task 3). The margin contract above applies unchanged; only the
// threshold moves.
func ContestCritAt(margin float64, roll dice.RollResult, bar float64) bool {
	if roll.StdDev > 0 {
		return margin/(roll.StdDev*math.Sqrt2) >= bar
	}
	// No usable scale to normalise against; fall back to the legacy
	// self-relative check rather than dividing by zero.
	return roll.ZScore >= bar
}
