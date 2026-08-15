package combat

// DefenceMitigation maps a defender's NORMALIZED margin onto the fraction of
// damage removed.
//
// Before U6 a defensive win was a clean miss and a spell deflection was a flat
// 0.5, which is two mechanisms answering one question. A bare win now mitigates
// 50%, rising linearly to 100% at ContestCritThreshold. Skill raises the margin,
// so skill raises mitigation continuously rather than in a step.
//
// A defensive CRIT is not this curve: it fully negates and fires the
// counterattack, and is resolved before this is reached.
//
// Applied AFTER item mitigation. There is no double count: a crit bypasses item
// mitigation and never receives a defence multiplier, because an attack crit
// beats a non-crit defence outright.
//
// The 0.5 and the crit threshold are STRUCTURAL, not tunables. 0.5 is the value
// that makes a bare win worth exactly half a swing, and the threshold is the
// point the curve has to meet so a defensive crit's full negation is continuous
// with it rather than a cliff. Moving either independently reintroduces the
// discontinuity this replaces, so neither is a config knob.
func DefenceMitigation(normalizedDefenceMargin float64) float64 {
	if normalizedDefenceMargin <= 0 {
		// Not a defensive win at all. Clamp to the bare floor rather than
		// extrapolating below it: a mis-signed margin must not be able to
		// AMPLIFY damage past 100%.
		return 0.5
	}
	if normalizedDefenceMargin >= ContestCritThreshold {
		return 1.0
	}
	return 0.5 + 0.5*(normalizedDefenceMargin/ContestCritThreshold)
}
