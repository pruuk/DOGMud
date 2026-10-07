package conditions

import "math"

// LightTrim is where an adjustable light record's output stands. It is an
// explicit state rather than a stored -Inf, so a save never has to encode an
// infinity.
type LightTrim string

const (
	LightFull    LightTrim = ""        // untrimmed: full strength
	LightTrimmed LightTrim = "trimmed" // running at LightOutput
	LightOff     LightTrim = "off"     // trimmed to nothing: the room was bright enough
)

// LightMax is the record's full strength: the applier's magnitude for a spell
// source, the authored number for an item. It reads whichever of
// light_strength and darkness_strength the spec declares (validateEffects
// refuses both), so one record shape and one trim state serve either
// polarity (lighting plan 5d, ruling D1); the caller knows which it holds
// from IsLightSource / IsDarknessSource. 0 when spec declares neither. A
// magnitude source added with no magnitude (an admin setcondition) is 0 and
// adds nothing; cast the spell instead.
func (b *Condition) LightMax(spec *ConditionSpec) float64 {
	if spec == nil {
		return 0
	}
	v, ok := spec.Effects[EffectLightStrength]
	if !ok {
		v, ok = spec.Effects[EffectDarknessStrength]
	}
	if !ok {
		return 0
	}
	if v.UsesMagnitude {
		return b.Magnitude
	}
	return v.Literal
}

// LightNow is the term this record adds to its room's light right now, and
// false when it adds none: expired, hooded, trimmed to nothing, or strengthless.
func (b *Condition) LightNow(spec *ConditionSpec) (float64, bool) {
	if b.Expired() || b.Hooded {
		return 0, false
	}
	max := b.LightMax(spec)
	if max <= 0 {
		return 0, false
	}
	switch b.LightTrim {
	case LightOff:
		return 0, false
	case LightTrimmed:
		// lightscale.Trim and TrimDarkness return Absent or a term in
		// (0, max], and SetLightOutput lands Absent on LightOff, so a trim
		// never stores a negative output or one above full strength. A
		// hand-edited or future save could, or the source's strength could
		// fall after the trim. Neither may enter the combine as stored.
		if b.LightOutput < 0 {
			return 0, false
		}
		return math.Min(b.LightOutput, max), true
	case LightFull:
		return max, true
	default:
		// An unrecognised state from a save is treated as full strength:
		// fail open, deliberately, so a bad save never snuffs a light.
		return max, true
	}
}

// SetLightOutput records a trim result from lightscale.Trim. Any non-finite
// output means the room needs nothing from this source and lands on LightOff:
// -Inf (lightscale.Absent), and equally +Inf and NaN.
func (b *Condition) SetLightOutput(out float64) {
	if math.IsInf(out, 0) || math.IsNaN(out) {
		b.LightTrim, b.LightOutput = LightOff, 0
		return
	}
	b.LightTrim, b.LightOutput = LightTrimmed, out
}

// ResetLight returns the record to full strength with any hood open: a fresh
// cast, a fresh equip, or unhood. Only AddConditionMagnitude calls it
// automatically. An equip path must call it itself (Character.Wear in
// internal/characters/worn.go does), and a plain AddCondition that revives an
// expired-but-unpruned record keeps that record's old hood and trim.
func (b *Condition) ResetLight() {
	b.LightTrim, b.LightOutput, b.Hooded = LightFull, 0, false
}

// LightSources returns every held, unexpired record whose spec declares a
// light strength, in held order. The order is stable, so a bearer's several
// sources always trim in the same sequence.
func (bs *Conditions) LightSources() []*Condition {
	var out []*Condition
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetConditionSpec(b.ConditionId); spec != nil && spec.IsLightSource() {
			out = append(out, b)
		}
	}
	return out
}

// DarknessSources returns every held, unexpired record whose spec declares a
// darkness strength, in held order (lighting plan 5d). It is LightSources'
// twin: a darkness is never in LightSources.
func (bs *Conditions) DarknessSources() []*Condition {
	var out []*Condition
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetConditionSpec(b.ConditionId); spec != nil && spec.IsDarknessSource() {
			out = append(out, b)
		}
	}
	return out
}

// LightAndDarknessSources returns every held, unexpired light or darkness
// record in one held order, for the one trim pass that solves both
// polarities in turn (internal/rooms.(*Room).TrimLightFor, lighting plan 5d).
func (bs *Conditions) LightAndDarknessSources() []*Condition {
	var out []*Condition
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetConditionSpec(b.ConditionId); spec != nil && (spec.IsLightSource() || spec.IsDarknessSource()) {
			out = append(out, b)
		}
	}
	return out
}
