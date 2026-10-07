package messaging

import "github.com/GoMudEngine/GoMud/internal/configs"

// The normal observer's band edges on the graded light scale, and the two
// numbers that bound how far an ability may move them.
//
// All three edges now have config knobs (LightBlindBelow, LightDimBelow, and
// plan 5b's LightDazzleAbove), and every function below takes them as
// arguments rather than reading config, so this file stays pure and testable.
const (
	// windowShiftCap is the most any ability may move the window down. It is
	// configs.LightWindowShiftCap, the one number the config validation and
	// the spell and potion magnitude caps also read.
	windowShiftCap = configs.LightWindowShiftCap
	// windowFloor is the light below which a shifted window reads nothing, no
	// matter how strong. Infra reach is independent of it: heat-sense reads
	// shapes at any light down to minus the reach (lighting plan 5c).
	windowFloor = 1
)

// SightThroughWindow reports what an observer reads at a given light level.
//
// strength moves every band edge DOWN by that many points, capped at
// windowShiftCap and floored at zero, so an ability trades bright-light comfort
// for dark-light acuity rather than simply gaining sight. reach is the separate
// heat-sensing extension: it reads SHAPES at any light down to the negation of
// reach, never faces, and only where the window itself reads worse (lighting
// plan 5c, the owner's ruling on 5b call 3).
//
// It takes the two lower band edges as arguments rather than reading config, so
// it stays a pure function with no locks and no global state. Its caller owns
// the config read.
func SightThroughWindow(light, strength, reach int, blindBelow, dimBelow int) SightDecision {
	strength = clampShift(strength)
	if reach < 0 {
		reach = 0
	}

	shiftedBlind := blindBelow - strength
	shiftedDim := dimBelow - strength

	if light >= shiftedDim {
		// Perfect and too-bright both read fully; SightThroughWindow does not
		// take a dazzle edge because dazzle never changes whether an
		// observer can make out a shape, only how much it costs them. Plan
		// 5b's cost lives in ComfortDistance and SightScoreMultiplier
		// (comfort.go, sight_mult.go); BandThroughWindow (band.go) and
		// LightTrimTarget below read the upper edge, to tell a player the
		// light hurts and to trim an adjustable light under it,
		// respectively.
		return SightFull
	}
	if light >= shiftedBlind && light >= windowFloor {
		return SightShapes
	}
	// Below the natural window. Infravision reads heat, not light, so it
	// gives shapes at ANY light down to minus its reach. It never yields
	// faces: natural sight has already won above wherever it reads fully.
	if reach > 0 && light >= -reach {
		return SightShapes
	}
	return SightNone
}

// ExitThroughWindow reports whether an observer sees THROUGH an exit into the
// next room at a given light. exitsAbove (Balance.LightExitsAbove) is the edge
// for normal eyes; night-vision strength moves it down exactly as it moves the
// blind and dim edges, clamped the same way. Infra reach plays no part in this
// light test; heat has its own path through an exit, shapes only
// (SensesHeatThroughExit, lighting plan 6).
// The caller still refuses first when the observer reads nothing at all here.
func ExitThroughWindow(light, strength, exitsAbove int) bool {
	return light >= exitsAbove-clampShift(strength)
}

// clampShift bounds an ability's window shift to [0, windowShiftCap]. Shared
// by SightThroughWindow, BandThroughWindow and ExitThroughWindow so they can
// never clamp differently.
func clampShift(strength int) int {
	if strength < 0 {
		return 0
	}
	if strength > windowShiftCap {
		return windowShiftCap
	}
	return strength
}

// LightTrimTarget is the brightest room light an observer with this
// night-vision strength reads without being dazzled: one point under the
// shifted dazzle edge. An adjustable light trims toward it (lighting plan 5a).
// One point, not half: a room at exactly 74.5 would round up to the edge.
// dazzleAbove is the caller's config knob (Balance.LightDazzleAbove via
// Lighting.DazzleAbove), a plan 5b knob rather than a package constant.
func LightTrimTarget(strength, dazzleAbove int) float64 {
	return float64(dazzleAbove - clampShift(strength) - 1)
}

// DarknessTrimTarget is the room light an adjustable darkness trims to
// (lighting plan 5d): one point inside the darkest light an observer can still
// use. The usable edge is minus the reach with infravision, where heat still
// reads shapes; otherwise it is the shifted blind edge, but never below
// windowFloor, where natural shapes need light at least that high
// (SightThroughWindow). One point inside, mirroring LightTrimTarget: a room
// parked exactly on the edge tips its bearer into the dark at the first
// downward drift in sky light (the 5d playtest), and a solved room within
// rounding of the edge could round past it. blindBelow is the caller's config
// knob (Lighting.BlindBelow).
func DarknessTrimTarget(strength, reach, blindBelow int) float64 {
	if reach > 0 {
		return float64(-reach + 1)
	}
	edge := blindBelow - clampShift(strength)
	if edge < windowFloor {
		edge = windowFloor
	}
	return float64(edge + 1)
}
