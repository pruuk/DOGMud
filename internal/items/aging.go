package items

import "math"

// AgingPhase represents a potion's lifecycle phase.
type AgingPhase int

const (
	PhaseFresh AgingPhase = iota
	PhaseFermented
	PhasePeak
	PhaseDeclining
	PhaseSpoiled
)

// AgingThresholds defines the round counts for each phase transition.
type AgingThresholds struct {
	FermentRounds int `yaml:"ferment_rounds,omitempty"`
	PeakRounds    int `yaml:"peak_rounds,omitempty"`
	DecayRounds   int `yaml:"decay_rounds,omitempty"`
	SpoilRounds   int `yaml:"spoil_rounds,omitempty"`
}

// HasAging returns true if aging thresholds are configured.
func (a AgingThresholds) HasAging() bool {
	return a.SpoilRounds > 0
}

// PeakPotency is the potency multiplier of the peak phase, a potion's
// strongest (GetAgingPhase).
const PeakPotency = 1.30

// Aging is one reading of a carried item's age: its phase, the potency that
// phase gives, and the elapsed rounds and effective speed
// GetPhaseDescription reads. Ages is false for an item that does not age (no
// aging thresholds, or never crafted): it reads fresh at full potency.
type Aging struct {
	Phase   AgingPhase
	Potency float64
	Elapsed uint64
	Speed   float64
	Ages    bool
}

// Spoiled reports a reading in the spoiled phase.
func (a Aging) Spoiled() bool {
	return a.Ages && a.Phase == PhaseSpoiled
}

// AgingSpeed is this item's effective aging speed: its own bottle, else its
// spec's, slowed by its maker's skill (CalcEffectiveAgingSpeed).
func (i *Item) AgingSpeed() float64 {
	bottle := i.BottleMultiplier
	if bottle <= 0 {
		bottle = i.GetSpec().BottleAgingMultiplier
	}
	return CalcEffectiveAgingSpeed(bottle, i.CraftSkill)
}

// AgingAt reads this item's age at round now, wherever it lies. Every reader
// of a potion's or a food's age asks this (or Character.PotionAging, which
// answers for the bandolier that holds it), so the rules live in one place.
//
// A Preserved item is spoiled. Preserved marks a potion while a preserving
// bandolier holds it (the Vitalis Bandolier, #794), and the bandolier's
// owner reads it at peak through Character.PotionAging instead. Anywhere
// else the potion has left the bandolier, and leaving it rots the potion at
// once (owner ruling 2026-11-06), whether or not its kind otherwise ages.
func (i *Item) AgingAt(now uint64) Aging {
	if i.Preserved {
		return Aging{Phase: PhaseSpoiled, Potency: 0, Speed: 1.0, Ages: true}
	}
	spec := i.GetSpec()
	if !spec.Aging.HasAging() || i.CraftedRound == 0 {
		return Aging{Phase: PhaseFresh, Potency: 1.0, Speed: 1.0}
	}
	var elapsed uint64
	if now >= i.CraftedRound {
		elapsed = now - i.CraftedRound
	}
	speed := i.AgingSpeed()
	phase, potency := GetAgingPhase(elapsed, spec.Aging, speed)
	return Aging{Phase: phase, Potency: potency, Elapsed: elapsed, Speed: speed, Ages: true}
}

// PeakAging is the reading a preserving bandolier holds a slotted potion at:
// peak potency, whether or not the potion's kind ages (a shop-bought potion
// that never ages counts as peak while it is held, owner ruling 2026-11-06).
// Elapsed is the first round of the peak window, so a description reads
// "plenty of time left".
func (i *Item) PeakAging() Aging {
	speed := i.AgingSpeed()
	if speed <= 0 {
		speed = 1.0
	}
	var elapsed uint64
	if th := i.GetSpec().Aging; th.HasAging() {
		elapsed = uint64(math.Ceil(float64(th.PeakRounds) / speed))
	}
	return Aging{Phase: PhasePeak, Potency: PeakPotency, Elapsed: elapsed, Speed: speed, Ages: true}
}

// PotionDurationMult is the multiplier a potion applies its conditions at:
// its aging potency, raised by its maker's skill. The drink path and the
// ambient bandolier both read it, so a held potion and a drunk one agree.
func PotionDurationMult(potency float64, craftSkill int) float64 {
	mult := potency
	if craftSkill > 0 {
		mult *= 1.0 + float64(craftSkill)/100.0
	}
	return mult
}

// GetAgingPhase returns the current phase and potency multiplier
// for a potion based on elapsed rounds and effective aging speed.
// effectiveSpeed combines bottle multiplier and craft skill:
// effectiveSpeed = bottleMultiplier * (1.0 - craftSkill/200)
func GetAgingPhase(elapsedRounds uint64, thresholds AgingThresholds, effectiveSpeed float64) (AgingPhase, float64) {
	if !thresholds.HasAging() {
		return PhaseFresh, 1.0
	}

	if effectiveSpeed <= 0 {
		effectiveSpeed = 1.0
	}

	// Higher effectiveSpeed = faster aging = shorter phase durations
	ferment := float64(thresholds.FermentRounds) / effectiveSpeed
	peak := float64(thresholds.PeakRounds) / effectiveSpeed
	decay := float64(thresholds.DecayRounds) / effectiveSpeed
	spoil := float64(thresholds.SpoilRounds) / effectiveSpeed

	elapsed := float64(elapsedRounds)

	switch {
	case elapsed < ferment:
		return PhaseFresh, 1.0
	case elapsed < peak:
		return PhaseFermented, 1.15
	case elapsed < decay:
		return PhasePeak, PeakPotency
	case elapsed < spoil:
		// Linear decline from 1.30 to 0.5
		progress := (elapsed - decay) / (spoil - decay)
		return PhaseDeclining, PeakPotency - (0.80 * progress)
	default:
		return PhaseSpoiled, 0.0
	}
}

// GetPhaseDescription returns descriptive text about a potion's age
// based on the viewer's alchemy skill level. No hard numbers shown.
func GetPhaseDescription(phase AgingPhase, alchemySkill int,
	elapsedRounds uint64, thresholds AgingThresholds,
	effectiveSpeed float64) string {

	if !thresholds.HasAging() {
		return ""
	}

	// Skill 0-5: no info
	if alchemySkill < 6 {
		return ""
	}

	// Compute how far into the current phase we are for urgency hints
	if effectiveSpeed <= 0 {
		effectiveSpeed = 1.0
	}
	decay := float64(thresholds.DecayRounds) / effectiveSpeed
	spoil := float64(thresholds.SpoilRounds) / effectiveSpeed
	peak := float64(thresholds.PeakRounds) / effectiveSpeed
	elapsed := float64(elapsedRounds)

	// Skill 6-15: general feel
	if alchemySkill <= 15 {
		switch phase {
		case PhaseFresh:
			return "It smells freshly brewed."
		case PhaseFermented:
			return "This has aged nicely."
		case PhasePeak:
			return "It has a rich, potent aroma."
		case PhaseDeclining:
			return "It's past its prime."
		case PhaseSpoiled:
			return "It smells off."
		}
		return ""
	}

	// Skill 16-29: phase + direction
	if alchemySkill < 30 {
		switch phase {
		case PhaseFresh:
			return "It's still maturing, not yet at its best."
		case PhaseFermented:
			return "It's developing well, nearing its peak."
		case PhasePeak:
			return "It's at peak potency."
		case PhaseDeclining:
			return "It's beginning to fade. Use it soon."
		case PhaseSpoiled:
			return "It has turned. Drinking this would be unwise."
		}
		return ""
	}

	// Skill 30+: exact phase + urgency
	switch phase {
	case PhaseFresh:
		return "Freshly brewed and still maturing. Give it time."
	case PhaseFermented:
		return "Fermenting nicely. It will reach peak soon."
	case PhasePeak:
		remaining := decay - elapsed
		total := decay - peak
		if total > 0 && remaining/total > 0.5 {
			return "At peak potency with plenty of time left."
		}
		return "At peak potency, but it won't last much longer."
	case PhaseDeclining:
		remaining := spoil - elapsed
		total := spoil - decay
		if total > 0 && remaining/total > 0.5 {
			return "Past its peak and declining, but still usable for now."
		}
		return "Declining rapidly. Use it very soon or distill it."
	case PhaseSpoiled:
		return "Completely spoiled. Only good for distilling remnants."
	}

	return ""
}

// CalcEffectiveAgingSpeed computes the combined aging speed from bottle
// multiplier and craft skill. Higher = faster aging.
func CalcEffectiveAgingSpeed(bottleMultiplier float64, craftSkill int) float64 {
	if bottleMultiplier <= 0 {
		bottleMultiplier = 1.0
	}
	// Higher craft skill slows aging: skill 30 = 15% slower
	skillFactor := 1.0 - float64(craftSkill)/200.0
	if skillFactor < 0.5 {
		skillFactor = 0.5 // Floor at 50% speed reduction
	}
	return bottleMultiplier * skillFactor
}

// IsSpoiledFood reports whether this food has gone bad at round now: its
// spec has aging thresholds, it was made at a known round, and it is past
// its spoil point. Food has no bottle, so only the cook's skill slows it.
// The one check player and mob eat both ask (#277).
func (i *Item) IsSpoiledFood(now uint64) bool {
	spec := i.GetSpec()
	if !spec.Aging.HasAging() || i.CraftedRound == 0 {
		return false
	}
	var elapsed uint64
	if now >= i.CraftedRound {
		elapsed = now - i.CraftedRound
	}
	phase, _ := GetAgingPhase(elapsed, spec.Aging, CalcEffectiveAgingSpeed(1.0, i.CraftSkill))
	return phase == PhaseSpoiled
}
