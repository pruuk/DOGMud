package lightscale

import "math"

// Trim returns the output an adjustable light source should run at: the
// smallest cut from its full strength that keeps the combined light at or
// below target.
//
// others is the combine of every other term the source joins, Absent (or 0)
// when there is none. max and the result are light-scale terms fed to Combine.
//
// The result solves Combine(others, out) == target on the linear sum,
// B(out) = B(target) - B(others), analytically; in floating point to rounding,
// capped at max. A linear "target - others" is wrong here: adding a source
// does not add its value in points. With no other light (others Absent, or at
// or below 0) the result is min(target, max). A target of +Inf is unbounded:
// the result is max. A target whose brightness overflows a float64 is solved
// in the log domain, so it still lands on the target.
//
// 🔑 The result is never a negative light: it is either Absent or a term in
// (0, max]. It is Absent, meaning "switch this source off" rather than "lit
// at nothing", when:
//
//   - target is at or below 0, so no light is wanted at all;
//   - max is at or below 0, a strengthless source with nothing to give (the
//     caller decides what a strengthless source's record should say;
//     rooms.TrimLightFor leaves it at full strength);
//   - others already reaches target without this source;
//   - the term needed has no brightness at all (float rounding);
//   - target or max is NaN, which cannot produce a meaningful term.
//
// It is the one solve for both polarities (lighting plan 5d, ruling D2): a
// darkness source solves the same equation on the darkness combine through
// TrimDarkness, which supplies the floor a low target needs.
func Trim(step, others, max, target float64) float64 {
	if math.IsNaN(target) || math.IsNaN(max) || !(target > 0) || !(max > 0) {
		return Absent()
	}
	step = coerceStep(step)
	if math.IsInf(others, 1) {
		// Unboundedly bright already: it reaches any target, even +Inf.
		return Absent()
	}
	if !present(others) || !(others > 0) {
		return math.Min(target, max)
	}
	if others >= target {
		return Absent()
	}
	if math.IsInf(target, 1) {
		return max
	}
	var need float64
	if bt := brightness(step, target); !math.IsInf(bt, 1) {
		need = level(step, bt-brightness(step, others))
	} else {
		// B(target) overflows, so B(target) - B(others) is solved relative
		// to the target in the log domain: the minus one of each B is far
		// below the precision of B(target) at this magnitude.
		need = target + step*math.Log1p(-math.Exp2((others-target)/step))/math.Ln2
	}
	if !(need > 0) {
		return Absent()
	}
	return math.Min(need, max)
}

// TrimDarkness returns the output an adjustable darkness source should run at
// so the room stays at or above floor: the least cut from its full strength
// that keeps light - Combine(otherDark, out) >= floor.
//
// light is the room's combined light (Absent or NaN reads 0, an unlit cave;
// +Inf is an unbounded light no darkness can pull down to floor, so the
// source runs at max), otherDark the combine of every other darkness in the
// room (Absent or 0 when there is none), max the source's full strength and
// floor its bearer's trim target, one point inside the bottom of the bearer's
// usable range (messaging.DarknessTrimTarget).
//
// Keeping the room at or above floor means Combine(otherDark, d) <= light -
// floor, which is Trim with others = otherDark and target = light - floor:
// darkness sources combine among themselves on the same linear sum lights do
// (lighting plan 5d, owner decision 1; plan 6). A budget at or below 0 means
// the room already sits at or below floor without this source, so it is not
// needed and the result is Absent. That is the caller-side floor Trim's low
// targets need, applied once here. Like Trim, the result is Absent or a term
// in (0, max], never a negative darkness.
func TrimDarkness(step, light, otherDark, max, floor float64) float64 {
	if math.IsNaN(floor) || math.IsNaN(max) {
		return Absent()
	}
	if math.IsNaN(light) || math.IsInf(light, -1) {
		light = 0
	}
	budget := light - floor
	if math.IsNaN(budget) || !(budget > 0) {
		return Absent()
	}
	return Trim(step, otherDark, max, budget)
}
