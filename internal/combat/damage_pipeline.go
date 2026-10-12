package combat

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// DamageChannel represents the type of damage being dealt.
type DamageChannel int

const (
	ChannelPhysical   DamageChannel = iota // Melee/ranged — scales with Strength
	ChannelMagical                         // Spells — scales with Willpower
	ChannelConviction                      // Social combat — scales with Charisma
)

// SkillMultiplier returns the damage multiplier for a given skill rank.
// Uses a sqrt curve: mult = base + (max - base) * sqrt(rank / softCap)
// Rank 0 → base (1.0), Rank 50 → max (3.0)
func SkillMultiplier(rank int) float64 {
	bal := configs.GetBalanceConfig()
	base := float64(bal.SkillMultiplierBase)
	max := float64(bal.SkillMultiplierMax)
	softCap := float64(bal.SkillSoftCap)

	if softCap <= 0 {
		softCap = 50.0
	}
	if rank <= 0 {
		return base
	}

	r := float64(rank)
	if r > softCap {
		r = softCap
	}

	return base + (max-base)*math.Sqrt(r/softCap)
}

// DamageScale returns the configured damage scale for a given channel.
// All channels use the same formula: stat × SkillMult × itemMult × scale.
// Physical default is 0.30 (stats ~100, so 100×1.0×1.0×0.30 = 30 raw).
// Magical / Conviction defaults are 1.0 (flat multipliers).
func DamageScale(channel DamageChannel) float64 {
	bal := configs.GetBalanceConfig()
	switch channel {
	case ChannelPhysical:
		return float64(bal.MeleeDamageScale)
	case ChannelMagical:
		return float64(bal.SpellDamageScale)
	case ChannelConviction:
		return float64(bal.RhetoricDamageScale)
	default:
		return 1.0
	}
}

// CalcRawDamage computes raw damage before mitigation and variance.
// All channels use the same unified formula, which has five factors:
//
//	raw = stat × SkillMultiplier(rank) × itemMult × ChannelScale × GlobalDamageMultiplier
//
// GlobalDamageMultiplier is a master knob over every channel. Both it and the
// per-channel scale are read from the balance config on each call, so neither
// is a constant. The Go fallbacks are Physical 0.30, Magical 1.00,
// Conviction 1.00 and global 1.00, but config.yaml routinely overrides all
// four; read it rather than quoting the fallbacks as live values.
func CalcRawDamage(stat int, skillRank int, itemMult float64, channel DamageChannel) float64 {
	if itemMult <= 0 {
		itemMult = 0.30 // fallback to unarmed-level multiplier
	}
	scale := DamageScale(channel)
	statFactor := float64(stat)

	globalMult := float64(configs.GetBalanceConfig().GlobalDamageMultiplier)
	return statFactor * SkillMultiplier(skillRank) * itemMult * scale * globalMult
}

// ResourceMultiplier returns a smooth penalty multiplier based on how depleted
// a resource pool is. Returns 1.0 at full, decreasing to (1 - penaltyMax) at 0%.
// Uses a quadratic curve (configurable exponent) so penalties are barely
// noticeable above 50% but ramp up as the pool empties.
func ResourceMultiplier(current, max int, penaltyMax float64) float64 {
	if max <= 0 {
		return 1.0
	}
	ratio := float64(current) / float64(max)
	if ratio >= 1.0 {
		return 1.0
	}
	if ratio < 0.0 {
		ratio = 0.0
	}
	curve := float64(configs.GetBalanceConfig().ResourcePenaltyCurve)
	return 1.0 - penaltyMax*math.Pow(1.0-ratio, curve)
}

// ApplyMitigation reduces raw damage by raw mitigation bent through curve
// (#465): final = raw x (1 - EffectiveMitigation(mitigationPct, curve)).
// mitigationPct is the uncapped fraction Character.Get*Mitigation returns,
// after any armor piercing has scaled it; the curve, not the caller, bounds it.
func ApplyMitigation(rawDmg float64, mitigationPct float64, curve MitigationCurve) float64 {
	return rawDmg * (1.0 - EffectiveMitigation(mitigationPct, curve))
}

// EffectiveMitigationFor is the fraction of a channel's damage char actually
// stops: its raw sum (Character.Get*Mitigation) bent through the channel's
// curve. The reader for anything that describes or weighs a character's
// protection (PowerScore, the status sheet), so it sees what combat applies.
// An unknown channel reads physical, as MitigationCurveFor does.
func EffectiveMitigationFor(char *characters.Character, channel DamageChannel) float64 {
	switch channel {
	case ChannelMagical:
		return EffectiveMitigation(char.GetMagicalMitigation(), MitigationCurveFor(channel))
	case ChannelConviction:
		return EffectiveMitigation(char.GetConvictionMitigation(), MitigationCurveFor(channel))
	default:
		return EffectiveMitigation(char.GetPhysicalMitigation(), MitigationCurveFor(ChannelPhysical))
	}
}

// MitigationCurveFor returns the configured mitigation curve for a damage
// channel (#465), four knobs per channel. An unknown channel reads the
// physical curve, so a caller that falls through still meets a real curve.
func MitigationCurveFor(channel DamageChannel) MitigationCurve {
	bal := configs.GetBalanceConfig()
	switch channel {
	case ChannelMagical:
		return MitigationCurve{
			Knee: float64(bal.MagicalMitigationKnee),
			Cap:  float64(bal.MagicalMitigationCap),
			Aim:  float64(bal.MagicalMitigationAim),
			Bend: float64(bal.MagicalMitigationBend),
		}
	case ChannelConviction:
		return MitigationCurve{
			Knee: float64(bal.ConvictionMitigationKnee),
			Cap:  float64(bal.ConvictionMitigationCap),
			Aim:  float64(bal.ConvictionMitigationAim),
			Bend: float64(bal.ConvictionMitigationBend),
		}
	default:
		return MitigationCurve{
			Knee: float64(bal.PhysicalMitigationKnee),
			Cap:  float64(bal.PhysicalMitigationCap),
			Aim:  float64(bal.PhysicalMitigationAim),
			Bend: float64(bal.PhysicalMitigationBend),
		}
	}
}

// CalcReachAdjustedItemMult returns the weapon's effective damage
// multiplier with the reach-utility factor applied for the
// attacker's current Position. Use this everywhere CalcRawDamage's
// itemMult argument was previously fed weaponSpec.DamageMultiplier
// directly.
//
// For attackers in non-grapple positions, ReachUtility returns 1.0
// and the result equals weaponSpec.DamageMultiplier — zero behavior
// change. For attackers in grapples, long weapons (reach exceeding
// the position's radius) take a multiplicative penalty.
//
// Natural-attack call paths (mob fist/claws/bite) compose directly:
//
//	naturalReach := items.ResolveNaturalReach(subtype)
//	posRadius := combat.PositionReachRadius(attacker.Position.State())
//	mult := combat.ReachUtility(naturalReach, posRadius)
func CalcReachAdjustedItemMult(
	weapon items.Item,
	attacker *characters.Character,
) float64 {
	// GetSpec() returns by value; handle unarmed (ItemId == 0) by
	// falling back to a 1.0 base multiplier.
	if weapon.ItemId == 0 {
		return 1.0
	}
	spec := weapon.GetSpec()
	baseMult := spec.DamageMultiplier
	if baseMult == 0 {
		baseMult = 1.0 // weapons without explicit multiplier default to 1×
	}
	if attacker == nil || attacker.Position == nil {
		return baseMult
	}
	reach := items.ResolveReach(&spec)
	posRadius := PositionReachRadius(attacker.Position.State())
	return baseMult * ReachUtility(reach, posRadius)
}

// GetConvictionDamageDescription converts conviction damage to descriptive
// text, as the one dealing it and any onlooker read it ("their resolve").
func GetConvictionDamageDescription(damageAmount int, targetMaxConviction int) string {
	return convictionDamageDescription(damageAmount, targetMaxConviction, "their")
}

// GetConvictionDamageDescriptionToTarget is the same band as the one who
// took the damage reads it on their own line: "your resolve", not "their
// resolve" (#262).
func GetConvictionDamageDescriptionToTarget(damageAmount int, targetMaxConviction int) string {
	return convictionDamageDescription(damageAmount, targetMaxConviction, "your")
}

// GetWordlessConvictionDamageDescription is GetConvictionDamageDescription for
// a taunt made without words (a beast's snarl, any howl, M6 slice 3): the
// bands that name an insult, a rebuke, a verbal assault or a tirade read as
// fear and dread instead.
func GetWordlessConvictionDamageDescription(damageAmount int, targetMaxConviction int) string {
	return convictionDamageBand(damageAmount, targetMaxConviction, "their", true)
}

// GetWordlessConvictionDamageDescriptionToTarget is the wordless band as the
// one who took the damage reads it ("your").
func GetWordlessConvictionDamageDescriptionToTarget(damageAmount int, targetMaxConviction int) string {
	return convictionDamageBand(damageAmount, targetMaxConviction, "your", true)
}

// convictionDamageDescription is the spoken band table; whose is the
// possessive the reader sees ("their" or "your").
func convictionDamageDescription(damageAmount int, targetMaxConviction int, whose string) string {
	return convictionDamageBand(damageAmount, targetMaxConviction, whose, false)
}

// convictionDamageBand is the one band table. wordless swaps the bands that
// name speech; the bands that name resolve, confidence and will are shared.
func convictionDamageBand(damageAmount int, targetMaxConviction int, whose string, wordless bool) string {
	if targetMaxConviction <= 0 {
		if wordless {
			return "a flicker of unease"
		}
		return "a mild rebuke"
	}

	pct := float64(damageAmount) / float64(targetMaxConviction) * 100

	switch {
	case pct < 5:
		return "a feeble jab at " + whose + " resolve"
	case pct < 15:
		if wordless {
			return "a jolt of fear"
		}
		return "a stinging insult"
	case pct < 30:
		if wordless {
			return "a wave of rattling dread"
		}
		return "a rattling verbal assault"
	case pct < 50:
		return "a crushing blow to " + whose + " confidence"
	case pct < 75:
		return "a devastating attack on " + whose + " will"
	default:
		if wordless {
			return "a soul-shattering terror"
		}
		return "a soul-shattering tirade"
	}
}
