package combat

// Chunk 5.11f — return ("reflect") damage is now mitigated by the attacker's
// matching defence channel.
//
// Found in the 5.11b playtest. Return damage is proportional to damage DEALT:
//
//	returnDmg = DamageToTarget * returnPct / 100
//	atkChar.Health -= returnDmg          // NO mitigation applied
//
// so raising the player's hit rate from 15% to 59.5% multiplied reflected damage
// by the same ~4x. Against a reflecting enemy SkillWeight lifted the player's
// output and the enemy's retaliation equally, which is exactly why the fight got
// faster without getting easier. And because the subtraction bypassed
// mitigation entirely, armour and Ironhide Brew did nothing about it.
//
// That also blocked the crit-damage slice: a crit lands at several times a
// normal hit, and an unmitigated quarter of that coming straight back would be
// lethal in a way nobody intended.
//
// Channels are matched to the source. Elemental species reflect heat and storm,
// which magical mitigation should answer; thorns enchantments and the Ironhide
// carapace are physical. See ReflectChannel.

// ReflectChannel identifies which of the attacker's mitigation pools answers a
// given source of return damage.
type ReflectChannel string

const (
	// ReflectPhysical is the DEFAULT. Thorns-style sources (the thornguard
	// enchantment, Ironhide Reflect Skin) answer to physical mitigation, and
	// defaulting here means existing content needs no data change.
	ReflectPhysical ReflectChannel = "physical"

	// ReflectMagical covers elemental backlash — the fire, magma, smoke and
	// storm elemental species.
	ReflectMagical ReflectChannel = "magical"
)

// ParseReflectChannel maps a data-file string onto a channel, defaulting to
// physical for the empty string and for anything unrecognised. Defaulting
// rather than erroring is deliberate: an unknown channel should degrade to the
// existing behaviour, not panic a boot on a typo in one species file.
func ParseReflectChannel(s string) ReflectChannel {
	if ReflectChannel(s) == ReflectMagical {
		return ReflectMagical
	}
	return ReflectPhysical
}

// SplitReflectPercents routes each reflect source to its channel.
//
// Equipment stat-mods (the thornguard enchantment) and the Ironhide Reflect Skin
// mutation are thorns — physical. Species reflect is routed by the species' own
// declared channel, which defaults to physical.
//
// Extracted as a pure function because it is the part of the wiring most likely
// to be wrong, and the call site in applyCombatDamageBonuses is awkward to reach
// from a unit test.
func SplitReflectPercents(statModPct, mutationPct, speciesPct int, speciesChannel ReflectChannel) (physical, magical int) {
	physical = statModPct + mutationPct

	if speciesChannel == ReflectMagical {
		magical = speciesPct
	} else {
		physical += speciesPct
	}

	return physical, magical
}

// ReflectDamage computes the return damage for one channel, after the
// attacker's mitigation for that channel.
//
// damageDealt is the damage the attacker just landed; returnPct is the
// defender's reflect percentage for this channel; mitigation is the ATTACKER's
// fraction for the matching channel (they are the one taking it).
//
// Returns 0 for a non-positive percentage so callers can sum channels without
// guarding each one.
func ReflectDamage(damageDealt float64, returnPct int, mitigation, cap float64) int {
	if returnPct <= 0 || damageDealt <= 0 {
		return 0
	}

	raw := damageDealt * float64(returnPct) / 100.0

	return int(ApplyMitigation(raw, mitigation, cap))
}
