package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// RunContest is THE entry point for every opposed contest in the game.
//
// It exists so that Balance.ContestFloor is read in exactly one place. Before
// U6 there were three wrapper pairs (maneuver, spell, global) over eight knobs,
// and because config.yaml shipped all three pairs at similar values, wiring a
// site to the wrong pair was invisible in production and became a live balance
// bug the moment one pair was retuned. One value removes the failure mode
// rather than guarding against it.
//
// The floor is passed down rather than read inside internal/contest, which is a
// config-free leaf by design.
//
// SCOPE: opposed contests only. Static-difficulty rolls -- search, track,
// forage, concentration -- are roadmap categories B and C and are deliberately
// unfloored. Do not route them here to "unify" them.
func RunContest(atkScore float64, entries []contest.Entry) contest.Result {
	// Compresses the DEFENCE entries, never atkScore: contest.Run derives the
	// roll spread from the attack score, so lowering it would shrink the spread
	// and cancel most of the compression. Applied HERE and nowhere else -- see
	// gap_compression_guard_test.go.
	entries = compressContestGap(atkScore, entries, contestGapSaturation())
	return contest.RunWithFloors(atkScore, entries, float64(configs.GetBalanceConfig().ContestFloor))
}

// compressContestGap narrows how far an attacker's score sits above each
// defence it is being rolled against, before either side is rolled.
//
//	compressedGap = gap * attack / (attack + k*gap)     when attack > defence
//	effectiveDefence = attack - compressedGap
//	effectiveDefence = defence                          otherwise
//
// WHY. Both rolls in a contest draw from ONE standard deviation taken from the
// attack score (contest.go:97,103), so the normalized margin is exactly
// N(gap/(0.15*A*sqrt2), 1) and crit fires when it clears a bar that floors at
// 1.5. That makes a 50% score edge a 52.8% crit rate, and a crit skips
// mitigation entirely. Compressing the gap pulls that back without touching
// parity.
//
// ⚠️ THE FORM IS SCALE-FREE ON PURPOSE, and that is the whole reason it is
// written this awkward way instead of the obvious `gap^p`. Divide through by A:
//
//	compressedGap/A = u / (1 + k*u)        where u = gap/A
//
// so the normalized margin depends ONLY on the ratio defence/attack. Two
// consequences, both of which an exponent form gets wrong:
//
//  1. MONOTONICITY. Raising your own score can never lower your win rate.
//     `gap^p` fails this: its normalized margin is (A-D)^p / (0.15*A*sqrt2),
//     whose derivative in A changes sign at A = D/(1-p). At p=0.8 that is
//     A = 5D, so a character at attack 455 facing defence 86 was already PAST
//     the peak -- measured, training Strength took the win rate from 0.784 down
//     to 0.725 by attack 5000, and crit from 17.8% to 10.9%. Getting stronger
//     made you worse.
//  2. TUNABILITY. `gap^p` mixes units (score^p against a spread in score), so
//     the same exponent is a mild nerf at newbie scores and a savage one at
//     veteran scores. It could not be tuned once. Every other knob in this
//     pipeline (ContestFloor, RollSpread, SkillMultiplier) is scale-free.
//
// ⚠️ It raises the DEFENCE rather than lowering the attack, and that is
// load-bearing. contest.Run derives the roll spread from the attack score it is
// handed and rolls the defender with it too. Lowering the attack would shrink
// the spread as well, and since crit is measured in units of that spread the
// compression would largely cancel itself: against a defence of 48, compressing
// the attack leaves crit at 94.3% where compressing the defence gives 28.7%.
// Moving the defence leaves atkScore, and therefore the spread, exactly as it is
// today. It also avoids a side effect nobody wanted -- a strong character's
// rolls becoming MORE consistent purely because they fought something weak.
//
// ⚠️ Only when the attacker is AHEAD of that particular defence. Symmetric
// compression would take an underdog from a 0.6% hit rate to 40.8%, which is a
// separate design decision about whether weak things can threaten strong ones
// and must not ride along with a crit fix.
//
// ⚠️ Each entry is compressed independently, so a mixed defence set keeps its
// internal ordering: a stronger defence stays stronger.
//
// ⚠️ SATURATING, so it barely touches near-even fights and bites hardest on
// blowouts -- which is the intent. At attack 455 a defence of 432 keeps a
// normalized margin of 0.209 against an uncompressed 0.238, while a defence of
// 86 falls from 3.824 to 1.169. An exponent form compressed BOTH by roughly
// half, flattening close fights that were never the problem.
//
// k == 0 returns the entries untouched, so the knob is a true no-op when unset.
func compressContestGap(atkScore float64, entries []contest.Entry, k float64) []contest.Entry {
	// ⚠️ `!(k > 0)`, NOT `k <= 0`. NaN fails every ordinary comparison, so
	// `k <= 0` is FALSE for NaN and would let it reach the arithmetic below,
	// turning every defence score into NaN. `Margin > 0` is then false forever
	// and the attacker silently never wins a contest anywhere in the game.
	// ConfigFloat is a bare float64 alias with no unmarshaller, so YAML `.nan`
	// really can arrive here. Guarded in the config reader too; this is the
	// pure function and must stand on its own.
	if !(k > 0) || len(entries) == 0 || !(atkScore > 0) {
		return entries
	}

	out := make([]contest.Entry, len(entries))
	copy(out, entries)
	for i := range out {
		gap := atkScore - out[i].Score
		if gap <= 0 {
			continue // attacker is not ahead of THIS defence; leave it alone
		}
		out[i].Score = atkScore - gap*atkScore/(atkScore+k*gap)
	}
	return out
}

var contestGapSaturationOverride *float64

func contestGapSaturation() float64 {
	if contestGapSaturationOverride != nil {
		return *contestGapSaturationOverride
	}
	k := float64(configs.GetBalanceConfig().ContestGapSaturation)
	// NaN fails every comparison, so test for it by the one property it has.
	// The validator already clamps, but this is the seam production reads and a
	// NaN reaching math here would make every defence NaN and silently stop the
	// attacker from ever winning a contest anywhere in the game.
	if !(k > 0) {
		return 0 // identity: unset, zero, negative, or NaN
	}
	return k
}
