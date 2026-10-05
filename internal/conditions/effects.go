package conditions

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
)

// EffectKind is one of the closed set of mechanical effects a record may
// declare. Combat reads them through Conditions.Effect. The set is closed on
// purpose: a new kind is a code change with a reader, never a data change.
type EffectKind string

const (
	EffectDamageMult     EffectKind = "damage_mult"     // physical damage multiplier (warcry)
	EffectDefenseMult    EffectKind = "defense_mult"    // defense score multiplier (rally, grapple exposure)
	EffectDodgeMult      EffectKind = "dodge_mult"      // dodge score multiplier (no producer today; kept for parity with the reader)
	EffectRegenMult      EffectKind = "regen_mult"      // multiplier on base health regen (heal spells, corpse feeding)
	EffectMitigationFlat EffectKind = "mitigation_flat" // flat physical mitigation points (wards)
	EffectPoolMaxPct     EffectKind = "pool_max_pct"    // fraction taken off a pool maximum; the pool rides on Condition.Source
	EffectAttacksCap     EffectKind = "attacks_cap"     // upper bound on swings per round
	// EffectNightVisionStrength is how far DOWN the scale an observer's usable
	// light band shifts. Aggregated as MAX, not summed: two night-sight
	// sources do not stack into a wider window than the better one grants.
	EffectNightVisionStrength EffectKind = `nightvision_strength`
	// EffectInfraReach is how far into the dark heat-sensing still reads
	// shapes: shapes at any light down to minus this many points on the light
	// scale (lighting plan 5c). Independent of strength: a creature can sense
	// heat deeply while being no better than anyone else at using faint
	// light. Effect() aggregates it by MAX (isMax) for any reader that calls
	// Effect(); Character.InfraReach does not, it log-sums every held
	// source's value through Conditions.EffectValues instead (owner ruling:
	// reach sources combine).
	EffectInfraReach EffectKind = `infra_reach`
	// EffectLightStrength is a light source's full strength on the light
	// scale: a literal for an item, "magnitude" for a spell cast at a scaled
	// strength (lighting plan 5a). It is NOT aggregated through Effect():
	// every held light record is its own term in the room's combine, read
	// through Conditions.LightSources. See light.go.
	EffectLightStrength EffectKind = `light_strength`
	// EffectDarknessStrength is a darkness source's full strength: light
	// taken away from the room, a literal for an item, "magnitude" for a
	// spell (lighting plan 5d, ruling D1). It is a light record with
	// darkening polarity: it shares the record's trim state (LightTrim,
	// LightOutput, ResetLight) and is read through LightMax and LightNow, but
	// IsLightSource stays false for it, so EmitsLight, hood, the as-lit end
	// line and every other light reader exclude it by construction. Read
	// through Conditions.DarknessSources, never Effect().
	EffectDarknessStrength EffectKind = `darkness_strength`
)

// AllEffectKinds is the closed set, for validation and docs.
var AllEffectKinds = []EffectKind{
	EffectDamageMult, EffectDefenseMult, EffectDodgeMult, EffectRegenMult,
	EffectMitigationFlat, EffectPoolMaxPct, EffectAttacksCap,
	EffectNightVisionStrength, EffectInfraReach, EffectLightStrength,
	EffectDarknessStrength,
}

func (k EffectKind) isMultiplier() bool {
	return k == EffectDamageMult || k == EffectDefenseMult || k == EffectDodgeMult || k == EffectRegenMult
}

func (k EffectKind) isCap() bool { return k == EffectAttacksCap }

// isMax reports whether Effect() aggregates this kind by taking the
// strongest held value, rather than summing. Used by NightVisionStrength,
// where summing would let two abilities stack into a window wider than
// either one grants. InfraReach is also isMax for Effect()'s own callers,
// but Character.InfraReach itself does not call Effect(): it reads
// Conditions.EffectValues and combines every source through
// lightscale.Combine instead (lighting plan 5c, owner ruling: reach sources
// combine, unlike nightvision strength).
func (k EffectKind) isMax() bool {
	return k == EffectNightVisionStrength || k == EffectInfraReach
}

// ScaledKinds are the effect kinds a spell or potion scales from its source
// (lighting plan 5c): a light's strength, nightvision's strength, infra's
// reach, and a darkness's strength (5d). A record carries one Magnitude, so a
// condition may declare at most one of them as "magnitude" (validateEffects
// refuses two).
var ScaledKinds = []EffectKind{EffectLightStrength, EffectNightVisionStrength, EffectInfraReach, EffectDarknessStrength}

// ScaledKind reports which of ScaledKinds this condition reads from its
// record's magnitude, if any.
func (b *ConditionSpec) ScaledKind() (EffectKind, bool) {
	for _, k := range ScaledKinds {
		if v, ok := b.Effects[k]; ok && v.UsesMagnitude {
			return k, true
		}
	}
	return "", false
}

// EffectValue is either a literal number or the word "magnitude", meaning the
// instance's own Magnitude, which the applier set.
type EffectValue struct {
	Literal       float64
	UsesMagnitude bool
}

func (v *EffectValue) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		if s == "magnitude" {
			*v = EffectValue{UsesMagnitude: true}
			return nil
		}
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return fmt.Errorf("effect value %q is neither a number nor the word magnitude", s)
		}
		*v = EffectValue{Literal: f}
		return nil
	}
	var f float64
	if err := unmarshal(&f); err != nil {
		return fmt.Errorf("effect value must be a number or the word magnitude: %w", err)
	}
	*v = EffectValue{Literal: f}
	return nil
}

func (v EffectValue) MarshalYAML() (interface{}, error) {
	if v.UsesMagnitude {
		return "magnitude", nil
	}
	return v.Literal, nil
}

// validateEffects refuses an unknown key, a magnitude-bound tick without a
// pool, a record reading more than one of ScaledKinds from its magnitude, a
// literal light_strength or darkness_strength of zero or less, a record
// declaring both (lighting plan 5d: one polarity per record), an adjustable
// record that declares neither, and a stacking record that is also a light
// or darkness source (a stack's summed magnitude is not a light strength, and
// AddConditionMagnitude takes the addStack path for a stacking spec, so a
// recast would never reset the light). It is called from
// ConditionSpec.Validate.
//
// The literal light rule is the one load-time range check here, and it is
// safe: LightNow refuses a strengthless magnitude record at runtime, so the
// two checks can never disagree about what counts as a light.
//
// It deliberately adds no range rule for EffectNightVisionStrength or
// EffectInfraReach. An EffectValue with UsesMagnitude set is not a number
// until AddConditionMagnitude or stack summation set the record's Magnitude
// at runtime, so a load-time check here could only ever catch a
// literal-declared instance, never a magnitude-declared one, which is
// partial coverage not worth the drift risk. The real clamp lives in
// SightThroughWindow (internal/messaging/window.go), which floors both
// numbers at zero and caps strength at windowShiftCap; a second copy here
// would only be able to disagree with that one, not replace it.
func (b *ConditionSpec) validateEffects() error {
	keys := make([]string, 0, len(b.Effects))
	for k := range b.Effects {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		known := false
		for _, ak := range AllEffectKinds {
			if EffectKind(k) == ak {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("conditionId %d (%s) declares unknown effect %q; see conditions.AllEffectKinds", b.ConditionId, b.Name, k)
		}
	}
	scaled := 0
	for _, k := range ScaledKinds {
		if v, ok := b.Effects[k]; ok && v.UsesMagnitude {
			scaled++
		}
	}
	if scaled > 1 {
		return fmt.Errorf("conditionId %d (%s) reads more than one of %v from its magnitude; a record carries one magnitude", b.ConditionId, b.Name, ScaledKinds)
	}
	if v, ok := b.Effects[EffectLightStrength]; ok && !v.UsesMagnitude && v.Literal <= 0 {
		return fmt.Errorf("conditionId %d (%s) declares light_strength %v; a light must be brighter than nothing", b.ConditionId, b.Name, v.Literal)
	}
	if v, ok := b.Effects[EffectDarknessStrength]; ok && !v.UsesMagnitude && v.Literal <= 0 {
		return fmt.Errorf("conditionId %d (%s) declares darkness_strength %v; a darkness must take some light away", b.ConditionId, b.Name, v.Literal)
	}
	if b.IsLightSource() && b.IsDarknessSource() {
		return fmt.Errorf("conditionId %d (%s) declares both light_strength and darkness_strength; a record has one polarity", b.ConditionId, b.Name)
	}
	if slices.Contains(b.Flags, Adjustable) && !b.IsLightSource() && !b.IsDarknessSource() {
		return fmt.Errorf("conditionId %d (%s) is adjustable but declares no light_strength or darkness_strength", b.ConditionId, b.Name)
	}
	if b.IsStacking() && b.IsLightSource() {
		return fmt.Errorf("conditionId %d (%s) is a stacking record and a light source; a stack's summed magnitude is not a light strength", b.ConditionId, b.Name)
	}
	if b.IsStacking() && b.IsDarknessSource() {
		return fmt.Errorf("conditionId %d (%s) is a stacking record and a darkness source; a stack's summed magnitude is not a darkness strength", b.ConditionId, b.Name)
	}
	if b.TickFromMagnitude {
		if b.TickPool == "" {
			return fmt.Errorf("conditionId %d (%s) sets tick_from_magnitude without tick_pool", b.ConditionId, b.Name)
		}
		if b.TickPercent != 0 {
			return fmt.Errorf("conditionId %d (%s) sets both tick_from_magnitude and tick_percent; the applier's magnitude IS the per-round amount", b.ConditionId, b.Name)
		}
	}
	return nil
}

// Effect combines every held, unexpired record's contribution for one kind:
// multipliers multiply (identity 1, a zero magnitude contributes nothing),
// flats and pool fractions sum (identity 0), attacks_cap takes the minimum
// (0 meaning no cap), and a max kind takes the strongest held value (identity
// 0). This is the ONE door timed state's mechanical effects are read through;
// combat was its first reader, and optics (the vision window) is now another.
// It never calls HasFlag with expire=true, which mutates.
func (bs *Conditions) Effect(kind EffectKind) float64 {
	if kind == EffectLightStrength || kind == EffectDarknessStrength {
		// Per-record, never aggregated: see LightSources and DarknessSources.
		return 0
	}
	product := 1.0
	sum := 0.0
	capValue := 0.0
	maxValue := 0.0
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		spec := GetConditionSpec(b.ConditionId)
		if spec == nil {
			continue
		}
		v, ok := spec.Effects[kind]
		if !ok {
			continue
		}
		val := v.Literal
		if v.UsesMagnitude {
			val = b.Magnitude
		}
		switch {
		case kind.isMultiplier():
			if val != 0 {
				product *= val
			}
		case kind.isCap():
			if val > 0 && (capValue == 0 || val < capValue) {
				capValue = val
			}
		case kind.isMax():
			if val > maxValue {
				maxValue = val
			}
		default:
			sum += val
		}
	}
	switch {
	case kind.isMultiplier():
		return product
	case kind.isCap():
		return capValue
	case kind.isMax():
		return maxValue
	default:
		return sum
	}
}

// EffectValues returns every held, unexpired record's value for one kind,
// magnitude-aware exactly as Effect reads it, in list order. It exists for a
// reader that combines values some other way than Effect's own rule:
// Character.InfraReach log-sums reach through lightscale.Combine (lighting
// plan 5c). Records that do not declare the kind contribute nothing.
func (bs *Conditions) EffectValues(kind EffectKind) []float64 {
	var out []float64
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		spec := GetConditionSpec(b.ConditionId)
		if spec == nil {
			continue
		}
		v, ok := spec.Effects[kind]
		if !ok {
			continue
		}
		val := v.Literal
		if v.UsesMagnitude {
			val = b.Magnitude
		}
		out = append(out, val)
	}
	return out
}

// HasEffect reports whether any held, unexpired record declares the kind.
func (bs *Conditions) HasEffect(kind EffectKind) bool {
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetConditionSpec(b.ConditionId); spec != nil {
			if _, ok := spec.Effects[kind]; ok {
				return true
			}
		}
	}
	return false
}
