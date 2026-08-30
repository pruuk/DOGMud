package combat

import (
	"math"

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
	return contest.RunWithFloors(atkScore, entries, float64(configs.GetBalanceConfig().ContestFloor))
}

// compressContestGap narrows how far an attacker's score sits above each
// defence it is being rolled against, before either side is rolled.
//
//	effectiveDefence = attack - (attack - defence) ^ p   when attack > defence
//	effectiveDefence = defence                            otherwise
//
// WHY. Both rolls in a contest draw from ONE standard deviation taken from the
// attack score (contest.go:97,103), so the normalized margin is exactly
// N((A-D)/(0.15*A*sqrt2), 1) and crit fires when it clears a bar that floors at
// 1.5. That makes a 50% score edge a 52.8% crit rate, and a crit skips
// mitigation entirely. Compressing the gap pulls that back without touching
// parity.
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
// p == 1.0 telescopes to attack - (attack - defence) = defence, an exact
// identity, and is the default.
func compressContestGap(atkScore float64, entries []contest.Entry, p float64) []contest.Entry {
	if p >= 1.0 || len(entries) == 0 {
		return entries
	}

	out := make([]contest.Entry, len(entries))
	copy(out, entries)
	for i := range out {
		gap := atkScore - out[i].Score
		if gap <= 0 {
			continue // attacker is not ahead of THIS defence; leave it alone
		}
		out[i].Score = atkScore - math.Pow(gap, p)
	}
	return out
}
