package combat

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Chunk 5.11g — skill-scaled crit damage.
//
// Before this, crit magnitude had no knob at all. Every channel implemented a
// crit as *bypass mitigation* and nothing else, which meant crit worth was set
// entirely by the DEFENDER's armour: 1.0x against an unarmoured target, 4.0x at
// the 75% mitigation cap. A maximally skilled attacker critting an unarmoured
// target landed a normal hit. Skill governed how OFTEN you critted (5.11d) but
// not what a crit was worth.
//
// The bypass is retained deliberately and this multiplies on top of it, so the
// damage a crit deals relative to a normal hit is
// `CritDamageMultiplier(skill) / (1 - mitigation)`. The bypass is what lets a
// crit answer heavy armour; the multiplier is what makes skill investment show
// up in the payoff.

// CritDamageMultiplier returns the multiplier applied to a critical hit's
// pre-mitigation damage, given the rank of the skill governing the attack's
// channel (weapon/unarmed/ranged-combat for melee, spellcasting for spells,
// rhetoric for taunts).
//
// The curve is LINEAR, not the sqrt used by SkillMultiplier, and that is the
// point: a sqrt curve puts a rank-25 attacker at roughly 75% of a rank-69
// attacker's value for 36% of the skill, which reintroduces the exact
// complaint chunk 5.7 opened with — investment past the middle stops paying.
//
// It also deliberately does NOT clamp at the skill soft cap, unlike
// SkillMultiplier. "Skill past the soft cap must do something" is the premise
// 5.11 exists to answer, and this is one of the two places it does.
//
// There is no runaway risk from rate and magnitude both scaling with skill:
// crit rate is bounded by 1.0, so damage-per-hit converges to this multiplier
// rather than compounding.
func CritDamageMultiplier(skillRank int) float64 {
	bal := configs.GetBalanceConfig()
	base := float64(bal.CritDamageBase)

	// A crit must never be worth less than the base. Mobs report rank 1 and
	// an unranked skill can report 0, so the floor is load-bearing, not
	// defensive padding.
	if skillRank <= 0 {
		return base
	}

	return base + float64(bal.CritDamagePerSkill)*float64(skillRank)
}

// OpeningStrikeMultiplier is the extra crit worth carried by the single opening
// strike of a surprise attack: the attacker's skullduggery expressed through the
// same crit-worth curve their combat skill already uses, times the ambush-only
// tuning knob.
//
// channelKnob is the per-channel ambush multiplier, passed by the caller rather
// than read here: melee uses SurpriseOpeningStrikeMultiplier, ranged uses
// SurpriseRangedStrikeMultiplier (Task 10), and the two ship at different values
// because a shot answers one fewer defence and already inherits the unengaged
// bonus.
//
// Returns 1.0 for a nil attacker so callers can multiply unconditionally.
func OpeningStrikeMultiplier(attacker *characters.Character, channelKnob float64) float64 {
	if attacker == nil {
		return 1.0
	}
	return CritDamageMultiplier(attacker.GetSkillLevel(skills.Skullduggery)) * channelKnob
}

// CritOrMitigatedDamageScaled is CritOrMitigatedDamage with an extra crit-only
// multiplier, used by the U10d ranged surprise shot.
//
// On a crit it bypasses mitigation entirely and scales by
// CritDamageMultiplier(skillRank) * bonusCritMult; on a normal hit it applies
// mitigation and bonusCritMult plays no part at all. Either way the
// multiplier is applied to the MEAN before the roll, because dice.RollStat
// derives its spread from the mean it is handed (stdDev = mean * RollSpread)
// — scaling the rolled result instead would stretch the spread by the
// multiplier and leave crits wildly swingier at high skill.
//
// bonusCritMult 0 reads as "unset" and means 1.0, matching AttackSide.Mult's
// convention. It applies ONLY on the crit branch: a surprise shot that lands
// as an ordinary hit is an ordinary hit.
//
// The melee channel deliberately does NOT use this. calcHitDamage carries
// backstab consumption and crit-buff bookkeeping, and floors at 0 rather than
// 1, so folding it in here would either lose behaviour or bloat the signature.
func CritOrMitigatedDamageScaled(rawDmg float64, skillRank int, isCrit bool, mitigPct, mitigCap, bonusCritMult float64) int {
	mean := rawDmg
	if isCrit {
		if bonusCritMult == 0 {
			bonusCritMult = 1.0
		}
		mean *= CritDamageMultiplier(skillRank) * bonusCritMult
	} else {
		mean = ApplyMitigation(rawDmg, mitigPct, mitigCap)
	}

	dmg := int(math.Round(dice.RollStat(mean).Value))
	if dmg < 1 {
		// A hit that lands must do something; 0 reads to the player as a bug.
		dmg = 1
	}
	return dmg
}

// CritOrMitigatedDamage rolls the damage for one spell or conviction hit, with
// no bonus multiplier. See CritOrMitigatedDamageScaled.
func CritOrMitigatedDamage(rawDmg float64, skillRank int, isCrit bool, mitigPct, mitigCap float64) int {
	return CritOrMitigatedDamageScaled(rawDmg, skillRank, isCrit, mitigPct, mitigCap, 1.0)
}
