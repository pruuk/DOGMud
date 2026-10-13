package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// SpellRawDamage is a damaging spell's mean before mitigation, from caster:
// the spell's primary stat and school skill through CalcRawDamage, the
// spell-power mutations, crash-site dampening, the caster weapon's spell
// multiplier and the conviction penalty. ok is false for a spell with no
// damage_multiplier (the legacy magnitude path) or no caster. Pure: the dice
// stay in hooks.rollSpellDamage, which calls this, as does the fight
// prediction (#301).
func SpellRawDamage(spellData *spells.SpellData, caster *characters.Character) (rawDmg float64, skillLevel int, ok bool) {
	if spellData == nil || caster == nil || spellData.DamageMultiplier <= 0 {
		return 0, 0, false
	}

	// U9: the spell's own primarystat and school drive damage, exactly as
	// they now drive its attack roll, duration and shield strength. This
	// site was missed by U9's original list of caster-side reads, which was
	// enumerated from spell_resolution.go only -- so for a while a spell
	// could roll to hit on one stat and deal damage on another.
	//
	// No shipped spell changes behaviour here: the fourteen charisma spells
	// are the manifestation family, and none of them carries a
	// DamageMultiplier.
	castSkill := skills.Spellcasting
	if spellData.HasSchool(spells.SchoolManifestation) {
		castSkill = skills.Manifestation
	}
	skillLevel = caster.GetSkillLevel(castSkill)
	rawDmg = CalcRawDamage(spellData.CasterStatValue(caster.Stats), skillLevel, spellData.DamageMultiplier, ChannelMagical)

	// Mutation graph: spell-power passives (Ether Gland, Corvid Brain) amplify caster output.
	if spBonus := mutations.GetSpellPowerMultiplier(caster.Mutations); spBonus != 0 {
		rawDmg *= 1.0 + spBonus
	}

	// #22 crash-site: inside the buried hull, belief-driven power is suppressed.
	// (caster is already non-nil, the guard above requires it.)
	if caster.HasConditionFlag(conditions.Dampened) {
		factor := float64(configs.GetBalanceConfig().CrashSiteSuppressionFactor)
		rawDmg *= factor
		if rawDmg < 1 {
			rawDmg = 1
		}
	}

	// Apply weapon spell damage multiplier (caster weapons), scaled
	// by gear-effectiveness for incorporeal casters.
	if caster.Equipment.Weapon.ItemId > 0 {
		if sdm := caster.Equipment.Weapon.GetSpec().SpellDamageMultiplier; sdm > 0 {
			rawDmg *= sdm * mutations.GearEffectivenessMultiplier(caster.Mutations)
		}
	}

	// Apply smooth conviction-based spell damage penalty. EffectivePoolMax,
	// not the raw max (U7 Task 11): a caster fielding a companion has their
	// current Conviction reserve-clamped already, so a raw denominator would
	// tax their spell damage on top of shrinking their pool.
	cpPenalty := float64(configs.GetBalanceConfig().ConvictionPenaltyMax)
	rawDmg *= ResourceMultiplier(caster.Conviction,
		caster.EffectivePoolMax(characters.PoolConviction), cpPenalty)

	return rawDmg, skillLevel, true
}

// SpellMitigation is target's mitigation against spellData's damage type,
// keyed by MitigationChannelFor: physical harm meets physical mitigation,
// mental meets magical, social meets conviction. A non-harm cast never
// reaches the damage pipeline. ok is false for a damage type with no channel
// (a non-harm spell); the curve is then the physical one, as rollSpellDamage
// has always fallen back to.
func SpellMitigation(spellData *spells.SpellData, target *characters.Character) (pct float64, curve MitigationCurve, channel string, ok bool) {
	ch, ok := MitigationChannelFor(spellData.DamageType)
	if !ok {
		return 0, MitigationCurveFor(ChannelPhysical), "none", false
	}
	switch ch {
	case ChannelPhysical:
		return target.GetPhysicalMitigation(), MitigationCurveFor(ch), "physical", true
	case ChannelMagical:
		return target.GetMagicalMitigation(), MitigationCurveFor(ch), "magical", true
	default:
		return target.GetConvictionMitigation(), MitigationCurveFor(ch), "conviction", true
	}
}
