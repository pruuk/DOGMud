package items

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/timber"
)

// Gear grades and bow woods (wilderness trades).
//
// A graded weapon, armour piece or shield works better or worse than its
// template, and a bow carries the traits of the wood its stave was split
// from. Both are applied in Item.GetSpec, on the copy it returns, so every
// reader (combat, weight, look) sees the same numbers and nothing is ever
// written back: the template and any Spec override stay as authored, and an
// enchantment or affix built on the override cannot compound a grade.
//
// Per grade (Balance Grade*, standard 1.0):
//
//	weapons      DamageMultiplier x GradeDamage*, SpeedMultiplier x GradeSpeed*
//	armour       Physical, Magical and Conviction mitigation and BlockRating x GradeArmor*
//	any of them  Weight x GradeWeight*
//
// Shipped: crude 0.85 damage and speed; pristine 1.25 damage, 1.15 speed and
// 0.90 weight; armour crude 0.85 to pristine 1.25.

// GradeMults are the multipliers a grade puts on gear.
type GradeMults struct {
	Damage, Speed, Weight, Armor float64
}

// GearGradeMults returns the multipliers for a grade. Ungraded and standard
// are all 1.0.
func GearGradeMults(q Quality) GradeMults {
	b := configs.GetBalanceConfig()
	switch q {
	case QualityCrude:
		return GradeMults{float64(b.GradeDamageCrude), float64(b.GradeSpeedCrude), float64(b.GradeWeightCrude), float64(b.GradeArmorCrude)}
	case QualityFine:
		return GradeMults{float64(b.GradeDamageFine), float64(b.GradeSpeedFine), float64(b.GradeWeightFine), float64(b.GradeArmorFine)}
	case QualitySuperb:
		return GradeMults{float64(b.GradeDamageSuperb), float64(b.GradeSpeedSuperb), float64(b.GradeWeightSuperb), float64(b.GradeArmorSuperb)}
	case QualityPristine:
		return GradeMults{float64(b.GradeDamagePristine), float64(b.GradeSpeedPristine), float64(b.GradeWeightPristine), float64(b.GradeArmorPristine)}
	}
	return GradeMults{1, 1, 1, 1}
}

// IsGearType reports whether items of type t are worn or wielded gear whose
// grade changes how they work: weapons, shields and every armour and jewelry
// slot. Lights and component bags are not.
func IsGearType(t ItemType) bool {
	switch t {
	case Weapon, Offhand, Head, Neck, Body, Belt, Gloves, Ring, Wrist, Back, Shoulders, Legs, Feet, Tail:
		return true
	}
	return false
}

// IsBow reports whether a spec is a bow: a shooting weapon that takes arrows.
func IsBow(spec ItemSpec) bool {
	return spec.Type == Weapon && spec.Subtype == Shooting && spec.AmmoTag == `arrows`
}

// IsShooter reports whether a spec is a ranged weapon that shoots (a bow,
// crossbow or sling): it wears per shot, never on a strike.
func IsShooter(spec ItemSpec) bool {
	return spec.Type == Weapon && spec.Subtype == Shooting
}

// scaleInt scales a whole-number rating, rounding half away from zero.
func scaleInt(v int, m float64) int {
	if v == 0 || m == 1 {
		return v
	}
	return int(math.Round(float64(v) * m))
}

// applyGrade scales spec for a graded instance of gear.
func applyGrade(spec ItemSpec, q Quality) ItemSpec {
	if !q.Valid() || q == QualityStandard || !IsGearType(spec.Type) {
		return spec
	}
	m := GearGradeMults(q)
	if spec.Type == Weapon {
		if spec.DamageMultiplier > 0 {
			spec.DamageMultiplier *= m.Damage
		}
		speed := spec.SpeedMultiplier
		if speed <= 0 {
			speed = 1.0 // the GetSpeedMultiplier default, so a grade still moves it
		}
		spec.SpeedMultiplier = speed * m.Speed
	}
	spec.PhysicalMitigation = scaleInt(spec.PhysicalMitigation, m.Armor)
	spec.MagicalMitigation = scaleInt(spec.MagicalMitigation, m.Armor)
	spec.ConvictionMitigation = scaleInt(spec.ConvictionMitigation, m.Armor)
	spec.BlockRating = scaleInt(spec.BlockRating, m.Armor)
	if spec.Weight > 0 {
		spec.Weight *= m.Weight
	}
	return spec
}

// applyCondition scales worn gear by Item.ConditionMult: weapon damage, and
// armour and shield protection.
func applyCondition(spec ItemSpec, mult float64) ItemSpec {
	if mult == 1 || !IsWearableGear(spec) {
		return spec
	}
	if spec.Type == Weapon {
		if spec.DamageMultiplier > 0 {
			spec.DamageMultiplier *= mult
		}
		return spec
	}
	spec.PhysicalMitigation = scaleInt(spec.PhysicalMitigation, mult)
	spec.MagicalMitigation = scaleInt(spec.MagicalMitigation, mult)
	spec.ConvictionMitigation = scaleInt(spec.ConvictionMitigation, mult)
	spec.BlockRating = scaleInt(spec.BlockRating, mult)
	return spec
}

// applyBowWood applies a bow's wood traits (speed and weight; accuracy is
// read at the shot, actions.ExecuteFire).
func applyBowWood(spec ItemSpec, wood string) ItemSpec {
	if wood == `` || !IsBow(spec) {
		return spec
	}
	t := timber.BowWood(wood)
	speed := spec.SpeedMultiplier
	if speed <= 0 {
		speed = 1.0
	}
	spec.SpeedMultiplier = speed * t.SpeedMult()
	if spec.Weight > 0 {
		spec.Weight *= t.WeightMult()
	}
	return spec
}

// woodSuffix names the wood of a bow, a bundle of arrows or a worked piece
// of timber: "Longbow (yew)".
func (i *Item) woodSuffix() string {
	if i.Wood == `` {
		return ``
	}
	return ` <ansi fg="item-quality">(` + timber.WoodName(i.Wood) + `)</ansi>`
}
