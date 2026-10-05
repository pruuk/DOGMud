package lightscale

import "math"

// Trim returns the output an adjustable light source should run at: the
// smallest cut from its full strength that keeps the combined light at or
// below target.
//
// others is the combine of every other term the source joins, Absent when
// there is none. max and the result are light-scale terms fed to Combine.
//
// The result solves Combine(others, out) == target analytically; in floating
// point to rounding, capped at max. A non-positive max is passed through
// unchanged, because 0 (or below) is a legitimate light term and the caller,
// not Trim, is responsible for refusing a strengthless source. The result is
// Absent when others already reaches target without this source, or when the
// term the arithmetic needs would fall below 0, the darkest light that occurs
// naturally: such a source would have to be darker than an unlit cave to
// matter, so it is not needed at all rather than "lit" at a meaningless
// negative value. A linear "target - others" is wrong here: on a log scale
// adding a source does not add its value. With others Absent the result is
// min(target, max).
//
// A NaN target or max cannot produce a meaningful term; Trim returns Absent
// rather than propagate the NaN.
//
// It is the one solve for both polarities (lighting plan 5d, ruling D2): a
// darkness source solves the same equation on the darkness combine through
// TrimDarkness, which supplies the floor a low target needs.
func Trim(step, others, max, target float64) float64 {
	if math.IsNaN(target) || math.IsNaN(max) {
		return Absent()
	}
	if !(step > 0) {
		step = 1
	}
	if !present(others) {
		return math.Min(target, max)
	}
	if others >= target {
		return Absent()
	}
	need := target + step*math.Log2(1-math.Exp2((others-target)/step))
	if need < 0 {
		return Absent()
	}
	return math.Min(need, max)
}

// TrimDarkness returns the output an adjustable darkness source should run at
// so the room stays at or above floor: the least cut from its full strength
// that keeps light - Combine(otherDark, out) >= floor.
//
// light is the room's combined light (Absent reads 0, an unlit cave),
// otherDark the combine of every other darkness in the room (Absent when
// there is none), max the source's full strength and floor its bearer's trim
// target, one point inside the bottom of the bearer's usable range
// (messaging.DarknessTrimTarget).
//
// Keeping the room at or above floor means Combine(otherDark, d) <= light -
// floor, which is Trim with others = otherDark and target = light - floor:
// darkness sources combine among themselves by the same halving rule lights
// do (lighting plan 5d, owner decision 1). A budget at or below 0 means the
// room already sits at or below floor without this source, so it is not
// needed and the result is Absent. That is the caller-side floor Trim's low
// targets need, applied once here.
func TrimDarkness(step, light, otherDark, max, floor float64) float64 {
	if math.IsNaN(floor) || math.IsNaN(max) {
		return Absent()
	}
	if !present(light) {
		light = 0
	}
	budget := light - floor
	if !(budget > 0) {
		return Absent()
	}
	return Trim(step, otherDark, max, budget)
}
