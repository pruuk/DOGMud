package buffs

import (
	"fmt"
	"sort"
	"strconv"
)

// EffectKind is one of the closed set of mechanical effects a record may
// declare. Combat reads them through Buffs.Effect. The set is closed on
// purpose: a new kind is a code change with a reader, never a data change.
type EffectKind string

const (
	EffectDamageMult     EffectKind = "damage_mult"     // physical damage multiplier (warcry)
	EffectDefenseMult    EffectKind = "defense_mult"    // defense score multiplier (rally, grapple exposure)
	EffectDodgeMult      EffectKind = "dodge_mult"      // dodge score multiplier (no producer today; kept for parity with the reader)
	EffectRegenMult      EffectKind = "regen_mult"      // multiplier on base health regen (heal spells, corpse feeding)
	EffectMitigationFlat EffectKind = "mitigation_flat" // flat physical mitigation points (wards)
	EffectPoolMaxPct     EffectKind = "pool_max_pct"    // fraction taken off a pool maximum; the pool rides on Buff.Source
	EffectAttacksCap     EffectKind = "attacks_cap"     // upper bound on swings per round
)

// AllEffectKinds is the closed set, for validation and docs.
var AllEffectKinds = []EffectKind{
	EffectDamageMult, EffectDefenseMult, EffectDodgeMult, EffectRegenMult,
	EffectMitigationFlat, EffectPoolMaxPct, EffectAttacksCap,
}

func (k EffectKind) isMultiplier() bool {
	return k == EffectDamageMult || k == EffectDefenseMult || k == EffectDodgeMult || k == EffectRegenMult
}

func (k EffectKind) isCap() bool { return k == EffectAttacksCap }

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

// validateEffects refuses an unknown key and a magnitude-bound tick without a
// pool. It is called from BuffSpec.Validate.
func (b *BuffSpec) validateEffects() error {
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
			return fmt.Errorf("buffId %d (%s) declares unknown effect %q; see buffs.AllEffectKinds", b.BuffId, b.Name, k)
		}
	}
	if b.TickFromMagnitude {
		if b.TickPool == "" {
			return fmt.Errorf("buffId %d (%s) sets tick_from_magnitude without tick_pool", b.BuffId, b.Name)
		}
		if b.TickPercent != 0 {
			return fmt.Errorf("buffId %d (%s) sets both tick_from_magnitude and tick_percent; the applier's magnitude IS the per-round amount", b.BuffId, b.Name)
		}
	}
	return nil
}

// Effect combines every held, unexpired record's contribution for one kind:
// multipliers multiply (identity 1, a zero magnitude contributes nothing),
// flats and pool fractions sum (identity 0), attacks_cap takes the minimum
// (0 meaning no cap). This is the ONE door combat reads timed state through.
// It never calls HasFlag with expire=true, which mutates.
func (bs *Buffs) Effect(kind EffectKind) float64 {
	product := 1.0
	sum := 0.0
	capValue := 0.0
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		spec := GetBuffSpec(b.BuffId)
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
		default:
			sum += val
		}
	}
	switch {
	case kind.isMultiplier():
		return product
	case kind.isCap():
		return capValue
	default:
		return sum
	}
}

// HasEffect reports whether any held, unexpired record declares the kind.
func (bs *Buffs) HasEffect(kind EffectKind) bool {
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetBuffSpec(b.BuffId); spec != nil {
			if _, ok := spec.Effects[kind]; ok {
				return true
			}
		}
	}
	return false
}
