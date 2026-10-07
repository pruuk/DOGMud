// Package lightscale holds the arithmetic of DOGMud's graded light scale.
//
// The scale runs -100 to 100 and is PERCEPTUAL. Zero is the darkest a place
// can be without active magical darkness, roughly an unlit cave; negative is
// magical darkness, light actively removed, and nothing else (lighting plan 6,
// owner ruling O1). One constant relates the scale to physical light: the
// doubling step, which is how many scale points twice as much light is worth.
//
// 🔑 Every operation works on LINEAR brightness, not on points (lighting plan
// 6, ruling O2). A light term of p points has brightness
//
//	B(p) = 2^(p/step) - 1
//
// and a brightness reads back as p(B) = step * log2(1 + B). B(0) is exactly 0,
// so "no light" and "a light of 0" are the same statement, and no sum or
// fraction of real light can read below 0. Darkness is the only way a room goes
// negative, and the room subtracts it in points; this package never does.
//
// The same step governs three things, which is why it is ONE config knob:
// combining sources, applying a sky fraction, and the shape of the daylight
// curve. See docs/superpowers/specs/2026-09-23-graded-room-lighting-amendment-celestial.md
// and docs/superpowers/specs/2026-10-07-lighting-plan-6-balance-design.md.
//
// This package is deliberately pure: no config reads, no globals, no locks. Its
// callers own the config read, the same discipline messaging.SightThroughWindow
// follows for the band edges.
package lightscale

import "math"

// Absent is the marker for a term that is not there at all: a sun below the
// horizon, a trimmed source switched off, an unlit fixture.
//
// Since lighting plan 6 it carries no arithmetic weight of its own. Combine and
// Attenuate read it exactly as a term of 0, because a term of 0 has brightness
// 0 and adds nothing. It survives as a MARKER, not a value: a trim result of
// Absent means "switch this source off" (conditions.SetLightOutput lands it on
// LightOff), and an unlit fixture records it so a look can still say "unlit".
// Combine and Attenuate never return it; a combination of nothing reads 0.
func Absent() float64 { return math.Inf(-1) }

// present reports whether a term should take part in a combination. Only finite
// values do; -Inf is Absent and NaN means a caller made an arithmetic mistake,
// which must not silently poison the whole room.
//
// ⚠️ +Inf is folded into "absent" too, which is deliberate but is NOT the same
// judgement as the other two. No caller can currently produce it, and if one
// ever does it is a bug upstream, most likely a division by zero. It is folded
// in here only because a term of infinite brightness has no sane reading on a
// bounded -100..100 scale. If a source whose magnitude is computed by division
// is ever added, give +Inf its own branch and make it loud.
func present(v float64) bool { return !math.IsInf(v, 0) && !math.IsNaN(v) }

// coerceStep turns a non-positive or NaN step into 1 rather than dividing by
// zero.
func coerceStep(step float64) float64 {
	if !(step > 0) {
		return 1
	}
	return step
}

// brightness is B(p) = 2^(p/step) - 1 for one light term. Every term that is
// not real light (Absent, NaN, +Inf, or at or below 0 points) reads 0. A light
// term cannot be negative: below 0 is darkness, and darkness is never a light
// term.
func brightness(step, p float64) float64 {
	if !present(p) || !(p > 0) {
		return 0
	}
	return math.Exp2(p/step) - 1
}

// level is p(B) = step * log2(1 + B), the inverse of brightness. A brightness
// at or below 0 reads 0.
func level(step, b float64) float64 {
	if !(b > 0) {
		return 0
	}
	return step * math.Log2(1+b)
}

// Combine returns the light produced by every term together: the sum of their
// brightnesses, read back in points.
//
//	combined = p( sum over i of B(t_i) )
//
// A single term reads its own value. Two equal terms read a little under one
// step brighter than one (52 and 52 read 59.9 at step 8), and the gap closes to
// a whole step as the terms grow; a term far below the brightest contributes
// almost nothing. Absent terms, and terms at or below 0, add nothing. A
// combination of nothing reads 0, never Absent.
func Combine(step float64, terms ...float64) float64 {
	step = coerceStep(step)
	sum := 0.0
	for _, t := range terms {
		sum += brightness(step, t)
	}
	return level(step, sum)
}

// Attenuate applies a transmission fraction to one light term: the share of the
// light that gets through a canopy, a roof, a drain-cap or a blizzard.
//
//	attenuated = p( fraction * B(light) )
//
// It never reads below 0, for any fraction. Well above the dim end a halving is
// very nearly one doubling step down, which is why canopy, roof and weather are
// one operator rather than three; near 0 the cut shrinks toward nothing,
// because there is almost no light left to take. A fraction at or below 0, or
// an Absent light, reads 0: no sky reaching here and no light are the same
// statement.
func Attenuate(step, light, fraction float64) float64 {
	if !present(light) || !(light > 0) || !(fraction > 0) {
		return 0
	}
	if fraction >= 1 {
		return light
	}
	step = coerceStep(step)
	return level(step, fraction*brightness(step, light))
}
