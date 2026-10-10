package combat

import "math"

// MitigationCurve turns a channel's raw mitigation (the sum of every source,
// as Character.Get*Mitigation returns it, uncapped) into the fraction of
// damage actually stopped (#465). It replaced a flat clamp at 0.75, which
// shipped content stacked far past, so every source above the clamp was
// wasted.
//
//   - Knee: up to here raw mitigation counts in full.
//   - Cap:  the hard ceiling; nothing stops more than this.
//   - Aim:  where the curve would level off without the cap, as an amount
//     above it. 0 only approaches the cap; above 0 the cap is reachable.
//   - Bend: how much raw mitigation past the knee it takes to cover half the
//     distance to Cap + Aim.
//
// Each channel reads its own four knobs (MitigationCurveFor). All four ship at
// 0.50 / 0.90 / 0.05 / 0.40 on every channel.
type MitigationCurve struct {
	Knee float64
	Cap  float64
	Aim  float64
	Bend float64
}

// EffectiveMitigation is the fraction of damage stopped by raw mitigation on
// curve c: linear up to the knee, diminishing above it, never above the cap.
//
//	raw <= 0     -> 0
//	raw <= knee  -> raw
//	otherwise    -> x = raw - knee
//	                min(knee + (cap + aim - knee) * x / (x + bend), cap)
//
// The curve is continuous at the knee and never decreasing. A NaN or negative
// raw reads as no mitigation (`!(raw > 0)`, the guard compressContestGap uses:
// NaN fails every comparison, so `raw <= 0` would let it through). A zero
// curve mitigates nothing; MitigationCurveFor never returns one, because the
// config validator fills every knob.
func EffectiveMitigation(raw float64, c MitigationCurve) float64 {
	if !(raw > 0) || !(c.Cap > 0) {
		return 0
	}
	if math.IsInf(raw, 1) {
		return c.Cap
	}
	if raw <= c.Knee {
		return math.Min(raw, c.Cap)
	}
	x := raw - c.Knee
	eff := c.Knee
	if c.Bend > 0 {
		eff += (c.Cap + c.Aim - c.Knee) * x / (x + c.Bend)
	} else {
		eff = c.Cap + c.Aim
	}
	return math.Min(eff, c.Cap)
}

// CritReduction is the share a critical hit loses to raw mitigation on curve
// c: the part of effective mitigation above the knee, 0 at or below it. A
// crit lands at raw x CritDamageMultiplier x (1 - CritReduction), so a target
// at or below the knee takes full crits, as before #465, and very heavy
// protection softens them. The defence multiplier stays skipped on crits.
func (c MitigationCurve) CritReduction(raw float64) float64 {
	return math.Max(0, EffectiveMitigation(raw, c)-c.Knee)
}
