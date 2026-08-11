package dice

import (
	"math/rand"
	"sync"
)

// Last-resort floors for opposed contests (roadmap chunk 5.9a).
//
// Combat has bounded both ends of its hit/avoid contest since the balance work
// that added MinAttackHitChance and MinDefenseChance: an outmatched attacker
// always keeps a puncher's chance, and an overwhelming one is never certain.
// Every OTHER consumer of OpposedRollStat -- stealth, theft, traps, detection --
// was unbounded, so a stat-100 thief against a stat-150 mark succeeded 0.9% of
// the time and a stat-200 thief against a stat-100 mark succeeded 99.1%.
//
// That difference was never decided. The floors simply lived in
// combat_helpers.go and nothing outside combat reached them.
//
// Set at startup from config via SetContestFloors, mirroring SetRollSpread.
// dice deliberately does not import configs.
var (
	minContestSuccess float64 = 0.05
	minContestResist  float64 = 0.05
	contestFloorLock  sync.RWMutex
)

// SetContestFloors configures the last-resort floors for floored opposed rolls.
//
// minSuccess: chance the initiator succeeds even after losing the roll.
// minResist:  chance the resister holds even after losing the roll.
//
// Both are clamped to [0, 0.5]. A value of 0 disables that end. Values at or
// above 0.5 would make the floor the dominant term rather than a last resort.
func SetContestFloors(minSuccess, minResist float64) {
	contestFloorLock.Lock()
	minContestSuccess = clampFloor(minSuccess)
	minContestResist = clampFloor(minResist)
	contestFloorLock.Unlock()
}

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

// ContestFloors reports the configured floors. For tests and diagnostics.
func ContestFloors() (minSuccess, minResist float64) {
	contestFloorLock.RLock()
	defer contestFloorLock.RUnlock()
	return minContestSuccess, minContestResist
}

// OpposedRollStatFloored is OpposedRollStat with both ends bounded.
//
// Use this for any contest where one side can be hopelessly outmatched and the
// result would otherwise be a foregone conclusion. Use plain OpposedRollStat
// only where the caller applies its own floors -- combat's resolveAttack does.
//
// When a floor flips the outcome, the margin is reduced to the smallest value
// carrying the new sign. A floor save is a BARE success, not a decisive one, and
// callers that scale an effect by margin must not read it as a rout.
func OpposedRollStatFloored(atk, def float64) (bool, float64, RollResult, RollResult) {
	contestFloorLock.RLock()
	floorSuccess, floorResist := minContestSuccess, minContestResist
	contestFloorLock.RUnlock()

	return OpposedRollStatFlooredWith(atk, def, floorSuccess, floorResist)
}

// OpposedRollStatFlooredWith is OpposedRollStatFloored with the floors supplied
// per call, for contests whose failure cost differs enough to want their own
// values.
//
// Spells are the motivating case: a fizzle costs the caster the round, and more
// than one round for a long cast, where a missed melee swing costs a fraction of
// a round. So spells floor at a lower value than combat despite protecting the
// same thing.
//
// Both floors are clamped to [0, 0.5] per call. Above that a floor stops being a
// last resort and becomes the dominant term.
func OpposedRollStatFlooredWith(atk, def, floorSuccess, floorResist float64) (bool, float64, RollResult, RollResult) {
	success, margin, attackRoll, defenseRoll := OpposedRollStat(atk, def)

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
