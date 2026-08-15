package dice

import (
	"math/rand"
)

// Last-resort floors for opposed contests (roadmap chunk 5.9a).
//
// Combat has bounded both ends of its hit/avoid contest since the balance work
// that added MinAttackHitChance and MinDefenseChance: an outmatched attacker
// always keeps a puncher's chance, and an overwhelming one is never certain.
// Every OTHER consumer -- stealth, theft, traps, detection -- used what is now
// OpposedRollStatRaw and was unbounded, so a stat-100 thief against a stat-150
// mark succeeded 0.9% of the time and a stat-200 thief against a stat-100 mark
// succeeded 99.1%.
//
// That difference was never decided. The floors simply lived in
// combat_helpers.go and nothing outside combat reached them. Chunk 5.10 then
// gave the floored roll the natural name so the safe path is the default one.
//
// U6 collapsed the config-driven route (combat.RunContest / Balance.ContestFloor)
// and this package-var route into the one config route; OpposedRollStat and
// OpposedRollStatWithFloors are deprecated, unreachable from production, and
// awaiting deletion alongside contest_floors_test.go's TestOpposedRollStat_*
// equivalence oracle (out of scope for this task). The floor below is now a
// fixed literal, not a settable package var.

// clampFloor keeps a floor a last resort rather than the dominant term.
func clampFloor(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 0.5 {
		return 0.5
	}
	return v
}

// OpposedRollStat performs a contested check between two stat-based scores with
// both ends floored. This is the DEFAULT opposed roll: use it for every
// attack-vs-defense, spell-vs-resist, stealth, theft, trap, grapple, bash, kick
// and trip check.
//
// Flooring both ends means neither a hopeless underdog nor an overwhelming
// favourite ever faces a foregone conclusion. If you believe you want the
// unfloored roll, see OpposedRollStatRaw -- and expect to justify it in
// contest_floor_guard_test.go.
//
// When a floor flips the outcome, the margin is reduced to the smallest value
// carrying the new sign. A floor save is a BARE success, not a decisive one, and
// callers that scale an effect by margin must not read it as a rout.
//
// Deprecated: use combat.RunWithGlobalFloors. Zero production callers as of U4;
// retained as the U4 equivalence oracle and for internal delegation, unreachable
// from production, and awaiting deletion alongside that oracle test. The 0.05
// literal below used to be the settable package-var pair (minContestSuccess,
// minContestResist); U6 collapsed that route into Balance.ContestFloor, read
// only by combat.RunContest, so this deprecated path no longer needs to be
// configurable.
func OpposedRollStat(atk, def float64) (bool, float64, RollResult, RollResult) {
	const floorSuccess, floorResist = 0.05, 0.05

	return OpposedRollStatWithFloors(atk, def, floorSuccess, floorResist)
}

// OpposedRollStatWithFloors is OpposedRollStat with the floors supplied per
// call, for contests whose failure cost differs enough to want their own
// values.
//
// Spells are the motivating case: a fizzle costs the caster the round, and more
// than one round for a long cast, where a missed melee swing costs a fraction of
// a round. So spells floor at a lower value than combat despite protecting the
// same thing.
//
// Both floors are clamped to [0, 0.5] per call. Above that a floor stops being a
// last resort and becomes the dominant term.
//
// Deprecated: use combat.RunWithGlobalFloors, combat.RunWithManeuverFloors or
// combat.RunWithSpellFloors. Zero production callers as of U4. U6 deletes it.
func OpposedRollStatWithFloors(atk, def, floorSuccess, floorResist float64) (bool, float64, RollResult, RollResult) {
	success, margin, attackRoll, defenseRoll := OpposedRollStatRaw(atk, def)

	floorSuccess = clampFloor(floorSuccess)
	floorResist = clampFloor(floorResist)

	switch {
	case !success && floorSuccess > 0 && rand.Float64() < floorSuccess:
		success, margin = true, 1
	case success && floorResist > 0 && rand.Float64() < floorResist:
		success, margin = false, -1
	default:
		return success, margin, attackRoll, defenseRoll
	}

	// Keep the returned rolls consistent with the flipped outcome; callers read
	// these flags as well as the boolean.
	attackRoll.Success, attackRoll.Margin = success, margin
	defenseRoll.Success, defenseRoll.Margin = !success, -margin

	return success, margin, attackRoll, defenseRoll
}
