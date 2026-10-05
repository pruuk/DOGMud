package timber

import "fmt"

// Wood traits (wilderness trades). A bow remembers the wood its stave was
// split from, and a bundle of arrows the wood of its shafts (items.Item.Wood).
// Each species gives its own small edge, authored in timber.yaml:
//
//	bow:   {speed: 1.05, weight: 0.95, accuracy: 1.04}
//	arrow: {damage: 1.05, accuracy: 1.02, recovery: 0.15}
//
// Multipliers are on the bow's speed and weight, on the aimed shot's attack
// score (accuracy) and on the shot's damage. Recovery is the chance a shot
// does not use up an arrow: it is picked up whole. Zero means neutral.

// BowTraits is what a wood gives a bow.
type BowTraits struct {
	Speed    float64 `yaml:"speed,omitempty"`
	Weight   float64 `yaml:"weight,omitempty"`
	Accuracy float64 `yaml:"accuracy,omitempty"`
}

// ArrowTraits is what a wood gives a bundle of arrows or bolts.
type ArrowTraits struct {
	Damage   float64 `yaml:"damage,omitempty"`
	Accuracy float64 `yaml:"accuracy,omitempty"`
	Recovery float64 `yaml:"recovery,omitempty"`
}

func mult(v float64) float64 {
	if v <= 0 {
		return 1.0
	}
	return v
}

// SpeedMult, WeightMult and AccuracyMult are the bow's multipliers (1.0 when
// unauthored).
func (b BowTraits) SpeedMult() float64    { return mult(b.Speed) }
func (b BowTraits) WeightMult() float64   { return mult(b.Weight) }
func (b BowTraits) AccuracyMult() float64 { return mult(b.Accuracy) }

// DamageMult and AccuracyMult are the arrows' multipliers (1.0 when
// unauthored).
func (a ArrowTraits) DamageMult() float64   { return mult(a.Damage) }
func (a ArrowTraits) AccuracyMult() float64 { return mult(a.Accuracy) }

func checkMult(name string, v float64) error {
	if v != 0 && (v < 0.5 || v > 1.5) {
		return fmt.Errorf("%s must be 0.5..1.5 (or omitted), got %v", name, v)
	}
	return nil
}

func (b BowTraits) validate() error {
	for name, v := range map[string]float64{`speed`: b.Speed, `weight`: b.Weight, `accuracy`: b.Accuracy} {
		if err := checkMult(name, v); err != nil {
			return err
		}
	}
	return nil
}

func (a ArrowTraits) validate() error {
	for name, v := range map[string]float64{`damage`: a.Damage, `accuracy`: a.Accuracy} {
		if err := checkMult(name, v); err != nil {
			return err
		}
	}
	if a.Recovery < 0 || a.Recovery > 0.9 {
		return fmt.Errorf("recovery must be 0..0.9, got %v", a.Recovery)
	}
	return nil
}

// WoodName is how a player reads a wood id ("black walnut"); "" for none.
// An id no longer in timber.yaml reads as itself.
func WoodName(id string) string {
	if id == `` {
		return ``
	}
	if sp := GetSpecies(id); sp != nil {
		return sp.Name
	}
	return id
}

// BowWood is the bow traits of a wood id (neutral for none or unknown).
func BowWood(id string) BowTraits {
	if sp := GetSpecies(id); sp != nil {
		return sp.Bow
	}
	return BowTraits{}
}

// ArrowWood is the arrow traits of a wood id (neutral for none or unknown).
func ArrowWood(id string) ArrowTraits {
	if sp := GetSpecies(id); sp != nil {
		return sp.Arrow
	}
	return ArrowTraits{}
}

// SpeciesForLog is the species whose felled trees give this log item, or nil.
func SpeciesForLog(itemId int) *Species {
	if itemId <= 0 {
		return nil
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, id := range current.order {
		if sp := current.species[id]; sp.LogItemId == itemId {
			return sp
		}
	}
	return nil
}
