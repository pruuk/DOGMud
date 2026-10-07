package configs

// LightWindowShiftCap is the most any ability may move an observer's sight
// window down the light scale: the ceiling on night-vision strength. It lives
// here, the lowest package every consumer imports, so the window model
// (internal/messaging's windowShiftCap), this file's validation of
// LightDefaultVisionStrength, and the spell and potion magnitude caps in
// internal/conditions and internal/items all read one number. It is a
// constant rather than a knob: the whole band model was balanced against it.
const LightWindowShiftCap = 24

// validateLighting sets defaults for the graded room lighting thresholds
// introduced by the graded lighting arc. It is a separate file from
// config.balance.combat.go's DARKNESS section because those knobs price the
// COMBAT penalty for fighting blind or by shapes, while these knobs define
// the light SCALE itself that Task 3's Room.LightLevel() and Task 4's
// ParticipantSight will read. Plans 2 and 3 add more knobs here (a dazzle
// threshold, ambient light by time of day, NightVision strength), so this
// area gets its own file up front rather than being folded into
// validateMisc and split out later.
func (b *Balance) validateLighting() {
	// LightBlindBelow and LightDimBelow are validated as a PAIR, following
	// the LightStarlight/LightMoonsFull precedent below: an inverted or
	// out-of-range pair reverts BOTH rather than leaving one knob correct
	// and the other wrong. See the struct field comment for why zero is
	// coerced rather than honoured for these two.
	blindInRange := b.LightBlindBelow >= -100 && b.LightBlindBelow <= 100 && b.LightBlindBelow != 0
	dimInRange := b.LightDimBelow >= -100 && b.LightDimBelow <= 100 && b.LightDimBelow != 0
	if !blindInRange || !dimInRange || b.LightBlindBelow >= b.LightDimBelow {
		b.LightBlindBelow = 25
		b.LightDimBelow = 50
	}

	// LightDazzleAbove: must sit above the dim edge and within the scale. Zero
	// means unset (a test binary never loads config.yaml). The fallback is NOT
	// the bare literal 75, following the LightExitsAbove precedent below: an
	// unconditional 75 can itself land below (or at) an operator's
	// legitimately elevated LightDimBelow (e.g. dim=80, or dim=100 leaves no
	// room above it at all), and a negative LightDimBelow (the never-blind
	// escape hatch) would let an absent dazzle key of 0 pass the range check
	// outright instead of falling back at all. The fallback is clamped above
	// LightDimBelow whenever 75 would not clear it; at dim=100 there is no
	// room for a comfortable band above the dim edge, so the fallback settles
	// for the one point still on the scale.
	dazzleInRange := b.LightDazzleAbove != 0 && b.LightDazzleAbove <= 100
	if !dazzleInRange || b.LightDazzleAbove <= b.LightDimBelow {
		fallback := ConfigInt(75)
		if fallback <= b.LightDimBelow {
			// dim=100 leaves no comfortable band; see the comment above.
			fallback = min(b.LightDimBelow+1, 100)
		}
		b.LightDazzleAbove = fallback
	}

	// LightExitsAbove is checked after the pair above so it sees the final,
	// valid LightBlindBelow rather than a value that is about to be
	// reverted. It gets its own range clamp plus the one cross-axis rule
	// that keeps it coherent: it must not sit below LightBlindBelow, or a
	// blind observer would see through an exit. It is deliberately NOT
	// required to sit above LightDimBelow; see the struct field comment.
	//
	// The fallback is NOT the bare literal 65: the whole-arc review found
	// that an unconditional 65 can itself land below an operator's
	// legitimately elevated LightBlindBelow (e.g. blind=90), reproducing the
	// exact contradiction this check exists to prevent. The fallback is
	// clamped up to LightBlindBelow whenever 65 would sit below it, so the
	// invariant "exits is never below blind" holds even after a revert, not
	// only for values that pass validation untouched.
	//
	// This clamps rather than reverting the whole group (the
	// LightBlindBelow/LightDimBelow revert-both style above) on purpose: the
	// blind/dim pair here was independently valid, and discarding it over an
	// unrelated exits typo would surprise an operator debugging their config
	// more than a single knob quietly self-correcting to the nearest valid
	// value. The fallback is 65 in the overwhelmingly common case (any
	// LightBlindBelow at or below 65, which includes every shipped default),
	// so this only changes behaviour for the unusual configs that raise
	// blind above 65.
	exitsInRange := b.LightExitsAbove >= -100 && b.LightExitsAbove <= 100 && b.LightExitsAbove != 0
	if !exitsInRange || b.LightExitsAbove < b.LightBlindBelow {
		exitsFallback := ConfigInt(65)
		if exitsFallback < b.LightBlindBelow {
			exitsFallback = b.LightBlindBelow
		}
		b.LightExitsAbove = exitsFallback
	}

	// LightRealMinimum, checked after the blind/dim pair so it sees the final
	// LightBlindBelow. Zero or negative is unset and takes the default 3. At
	// or above LightBlindBelow it is clamped to one point below it, the
	// LightExitsAbove precedent of clamping to the nearest valid value rather
	// than reverting; with LightBlindBelow at 1 or below (the never-blind
	// escape hatch) that leaves 0, no floor at all, which is the only value
	// that cannot let a sliver of light grant sight.
	if b.LightRealMinimum <= 0 {
		b.LightRealMinimum = 3
	}
	if b.LightRealMinimum >= b.LightBlindBelow {
		b.LightRealMinimum = max(b.LightBlindBelow-1, 0)
	}

	// LightDefaultVisionStrength is clamped to [0, 24] first, then zero
	// (whether authored directly or reached by clamping a negative) is
	// defaulted to 12. Doing it in that order means the accepted AUTHORED
	// range is effectively [1, 24], not [0, 24]: zero is not a value an
	// operator can choose, it is indistinguishable from the field being
	// unset, following the same idiom as ProgressMult (0 means "use the
	// default", not "shift by nothing"; see the struct field comment for
	// why a bare flag cannot mean a shift of zero).
	//
	// The upper bound is LightWindowShiftCap (24), the constant
	// internal/messaging's windowShiftCap is defined from. A
	// value above the cap is clamped down to 24 rather than reverted to the
	// default, matching how LightExitsAbove clamps rather than reverts for
	// its own out-of-range case above: the operator's intent (a strong
	// shift) is still honoured, just capped at the strongest the window
	// model can express.
	if b.LightDefaultVisionStrength < 0 {
		b.LightDefaultVisionStrength = 0
	}
	if b.LightDefaultVisionStrength > LightWindowShiftCap {
		b.LightDefaultVisionStrength = LightWindowShiftCap
	}
	if b.LightDefaultVisionStrength == 0 {
		b.LightDefaultVisionStrength = 12
	}

	// LightDoublingStep: non-positive is coerced, not honoured. Zero divides
	// by zero in the combine and makes every source identical.
	if !(b.LightDoublingStep > 0) {
		b.LightDoublingStep = 8
	}

	// WorldLatitude: zero means UNSET and is coerced, the same idiom as
	// LightDefaultVisionStrength. Out-of-range reverts.
	//
	// 🔴 An earlier draft had zero HONOURED, meaning "this world has no
	// latitude, fall back to Timing.NightHours". That was incoherent, and the
	// incoherence was not academic. Go cannot distinguish an unset float from
	// an authored zero, and until lighting plan 6 surfaced it, WorldLatitude
	// was absent from config.yaml, so the shipped configuration read a bare
	// zero. Honouring zero would therefore have shipped DOGMud at no
	// latitude: a flat eight-hour night, no seasons, and the entire celestial
	// model built and never once reached. Any config file that omits the key
	// is in the same position today.
	//
	// So zero is unset, and the NightHours fallback is deleted rather than
	// repaired. An operator who wants an equator-like world of twelve-hour
	// nights all year authors a latitude near zero, such as 0.001; there is no
	// longer any path that reaches Timing.NightHours for day length.
	if b.WorldLatitude < -90 || b.WorldLatitude > 90 || b.WorldLatitude == 0 {
		b.WorldLatitude = 46.5
	}

	// LightEquinoxNoon must sit on the scale. Zero is coerced: a world whose
	// equinox noon is as dark as an unlit cave is not a calibration, it is an
	// unset field.
	// The range is INCLUSIVE at both ends, matching LightStarlight and
	// LightMoonsFull below and LightBlindBelow/LightDimBelow above. An earlier
	// draft wrote `<= -100` here, which silently reverted an operator who
	// authored the scale floor while every sibling knob accepted it.
	if b.LightEquinoxNoon < -100 || b.LightEquinoxNoon > 100 || b.LightEquinoxNoon == 0 {
		b.LightEquinoxNoon = 70
	}

	// The moon anchors are validated as a PAIR, following the
	// LightBlindBelow/LightDimBelow pair above: starlight at or above the
	// full-moon value inverts the curve, so an invalid pair reverts BOTH
	// rather than leaving one correct.
	//
	// Unlike the LightBlindBelow/LightDimBelow pair, neither knob needs its own
	// `!= 0` unset guard, and that is not an oversight. An unset pair is (0, 0),
	// which the ordering check below already catches, because starlight is then
	// not strictly below the full-moon value. Adding the guard would be dead
	// code.
	starOK := b.LightStarlight >= -100 && b.LightStarlight <= 100
	fullOK := b.LightMoonsFull >= -100 && b.LightMoonsFull <= 100
	if !starOK || !fullOK || b.LightStarlight >= b.LightMoonsFull {
		b.LightStarlight = 10
		b.LightMoonsFull = 35
	}

	// Moon weights: a negative weight would make a waxing moon darken the sky,
	// so negatives are floored at zero. All three at zero leaves the moon curve
	// with no span at all, so that reverts the whole set rather than leaving a
	// sky that never changes.
	if b.LightMoonWeightSwiftmoon < 0 {
		b.LightMoonWeightSwiftmoon = 0
	}
	if b.LightMoonWeightWanderer < 0 {
		b.LightMoonWeightWanderer = 0
	}
	if b.LightMoonWeightEye < 0 {
		b.LightMoonWeightEye = 0
	}
	if b.LightMoonWeightSwiftmoon+b.LightMoonWeightWanderer+b.LightMoonWeightEye <= 0 {
		b.LightMoonWeightSwiftmoon = 4.0
		b.LightMoonWeightWanderer = 1.0
		b.LightMoonWeightEye = 0.5
	}

	// Light-spell scaling. Zero or negative is coerced: a zero divisor divides
	// by zero, and a test binary never loads config.yaml, so zero must mean
	// "unset" for the two bases too.
	for _, k := range []struct {
		v   *ConfigFloat
		def ConfigFloat
	}{
		{&b.LightSpellStrengthBase, 40}, {&b.LightSpellStrengthStatDivisor, 10}, {&b.LightSpellStrengthSkillDivisor, 2},
		{&b.LightSpellDurationBase, 2}, {&b.LightSpellDurationStatDivisor, 50}, {&b.LightSpellDurationSkillDivisor, 20},
		{&b.LightNightVisionSpellBase, 4}, {&b.LightNightVisionSpellStatDivisor, 12.5}, {&b.LightNightVisionSpellSkillDivisor, 6.5},
		{&b.LightInfraSpellBase, 5}, {&b.LightInfraSpellStatDivisor, 7}, {&b.LightInfraSpellSkillDivisor, 3},
		{&b.LightDarknessSpellStrengthBase, 40}, {&b.LightDarknessSpellStrengthStatDivisor, 10}, {&b.LightDarknessSpellStrengthSkillDivisor, 2},
		{&b.LightDarknessSpellDurationBase, 2}, {&b.LightDarknessSpellDurationStatDivisor, 50}, {&b.LightDarknessSpellDurationSkillDivisor, 20},
	} {
		if !(*k.v > 0) {
			*k.v = k.def
		}
	}

	// Infravision. A cap of zero divides by zero in the penalty ramp; above
	// 100 reaches past the scale. The floor is a multiplier in (0, 1].
	if b.LightInfraReachCap <= 0 || b.LightInfraReachCap > 100 {
		b.LightInfraReachCap = 50
	}
	if b.LightInfraPenaltyFloor <= 0 || b.LightInfraPenaltyFloor > 1.0 {
		b.LightInfraPenaltyFloor = 0.90
	}
}
