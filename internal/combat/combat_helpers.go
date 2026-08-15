package combat

import (
	"fmt"
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// unloadedMeleeDamageCap is the maximum effective damage multiplier a ranged
// (Shooting-subtype) weapon may contribute on the MELEE auto-attack swing path.
// Ranged weapons keep their full multiplier on the deliberate SHOOT path; in
// melee they are improvised clubs and clamp here.
const unloadedMeleeDamageCap = 0.30

// combatContext carries per-round environmental info into the combat engine.
type combatContext struct {
	sourceCanSee bool // source has nightvision OR room visibility >= 1
	targetCanSee bool // target has nightvision OR room visibility >= 1
	// Chunk 3.3: when true, the first hit of this round is guaranteed to
	// crit regardless of the z-score threshold. Set by the round dispatcher
	// when the defender was snapshotted as Sleeping at round start.
	// Uses combatContext (not a separate parameter) so it threads through
	// calculateCombat without touching Attack*Vs* signatures.
	forceCrit bool
}

// weaponSetup holds pre-computed weapon info for a single weapon swing.
type weaponSetup struct {
	weapon        items.Item
	weaponName    string
	weaponSubType items.ItemSubType
	attacks       int
	baseDmg       float64
	dmgVariance   float64
	critBuffs     []int
	weaponSpeed   float64
	weaponDmgMult float64
	isOffhand     bool
	penalty       int // dual wield penalty
}

// swingDamageParams holds per-swing damage values after pipeline calculations.
//
// There is deliberately NO variance field here. The spread of a damage roll is
// always derived from the mean actually being rolled, via dice.RollStat (which
// applies StdDevFor(mean) = mean * RollSpread). Carrying a pre-computed
// variance alongside two different means (dmgMean for a normal hit,
// rawDmgForCrit for a crit) is what let the crit roll inherit the mitigated
// mean's spread — roughly half the intended width against an armored target,
// and staler still once the statmod/health/prone/mutation/warcry modifiers
// below moved the means. See internal/dice/context.md and
// combat.ExecuteSkillMove, which has always rolled this way.
type swingDamageParams struct {
	dmgMean       float64
	rawDmgForCrit float64
	critDmgMult   float64 // chunk 5.11g: skill-scaled crit worth, applied to rawDmgForCrit only
	critBuffs     []int
	msgSeed       int
}

// bestDefenseResult holds the outcome of best-of-all defense resolution.
type bestDefenseResult struct {
	margin       float64
	defenseType  string
	hitRoll      dice.RollResult
	defRoll      dice.RollResult
	defenseFloor bool // true if defense succeeded via floor save
	floored      bool // the contest floor CHANGED this outcome; it must never crit
}

// calcSwingCount computes the number of swings for a single weapon per round.
// Merges dex + weapon speed + skill into one formula, replacing the old
// outer-loop calcAttackCount × inner-loop ws.attacks double multiplication.
//
// Formula: swings = max(1, round(1 + (dex - 50) / 100 × weaponSpeed × (1 + skill / softCap)))
// Then apply stamina, encumbrance, position, and recovery modifiers.
// Hard cap: 4 per weapon.
func calcSwingCount(sourceChar *characters.Character, weaponSpeed float64, extraAttacks int, isOffhand bool) int {
	bal := configs.GetBalanceConfig()
	softCap := float64(bal.SkillSoftCap)
	if softCap <= 0 {
		softCap = 50
	}

	dex := float64(sourceChar.GetEffectiveDexterity())
	skillLevel := float64(sourceChar.GetCombatSkillLevel()) * float64(bal.SkillWeight)

	// Core swing count formula
	swings := 1.0 + (dex-50.0)/100.0*weaponSpeed*(1.0+skillLevel/softCap)
	swings += float64(extraAttacks)

	// Offhand penalty: skill governs dual-wield speed
	if isOffhand {
		dualSkill := float64(sourceChar.GetSkillLevel(skills.WeaponCombat))
		if sourceChar.IsUnarmedStyle() {
			dualSkill = float64(sourceChar.GetSkillLevel(skills.UnarmedCombat))
		}
		dualSkill *= float64(bal.SkillWeight)
		dualWieldMod := 0.5 + (dualSkill/50.0)*0.5
		swings *= dualWieldMod
	}

	// Apply smooth stamina-based swing count penalty
	spPenalty := float64(bal.StaminaPenaltyMax)
	swings *= ResourceMultiplier(sourceChar.Stamina, sourceChar.StaminaMax.Value, spPenalty)

	// Apply encumbrance penalty (weight-based)
	carriedWeight := sourceChar.GetCarriedWeight()
	capacity := sourceChar.CarryCapacity()
	if carriedWeight > capacity {
		overAmount := carriedWeight - capacity
		overRatio := overAmount / capacity
		encumbrancePenalty := math.Min(overRatio*0.5, 0.5)
		swings *= (1.0 - encumbrancePenalty)
	}

	// Haste buff: significant attack speed boost
	if sourceChar.HasBuffFlag(buffs.Haste) {
		swings *= float64(bal.HasteSwingMultiplier)
	}

	// Position-based speed modifier (chunk 4b R1: FSM-driven via the new
	// helper; legacy CombatPosition.GetSpeedMultiplier is sunset in S5).
	swings *= sourceChar.GetPositionSpeedMultiplier()

	// Round to nearest int, minimum 1
	result := int(math.Round(swings))
	if result < 1 {
		result = 1
	}

	// Recovery penalty: force to 1
	if sourceChar.HasCondition(characters.ConditionRecoveryPenalty) {
		result = 1
	}

	// Hard cap: max 4 swings per weapon
	if result > 4 {
		result = 4
	}

	return result
}

// collectAttackWeapons gathers all weapons the character can attack with.
func collectAttackWeapons(sourceChar *characters.Character) []items.Item {
	attackWeapons := []items.Item{}

	if sourceChar.Equipment.Weapon.ItemId > 0 {
		attackWeapons = append(attackWeapons, sourceChar.Equipment.Weapon)
	}

	if sourceChar.Equipment.Offhand.ItemId > 0 && sourceChar.Equipment.Offhand.GetSpec().Type == items.Weapon {
		attackWeapons = append(attackWeapons, sourceChar.Equipment.Offhand)
	}

	// Extra arm weapons (from extra-arms mutation)
	if sourceChar.ExtraArms >= 1 && sourceChar.Equipment.ExtraArm1.ItemId > 0 && sourceChar.Equipment.ExtraArm1.GetSpec().Type == items.Weapon {
		attackWeapons = append(attackWeapons, sourceChar.Equipment.ExtraArm1)
	}
	if sourceChar.ExtraArms >= 2 && sourceChar.Equipment.ExtraArm2.ItemId > 0 && sourceChar.Equipment.ExtraArm2.GetSpec().Type == items.Weapon {
		attackWeapons = append(attackWeapons, sourceChar.Equipment.ExtraArm2)
	}
	if sourceChar.ExtraArms >= 3 && sourceChar.Equipment.ExtraArm3.ItemId > 0 && sourceChar.Equipment.ExtraArm3.GetSpec().Type == items.Weapon {
		attackWeapons = append(attackWeapons, sourceChar.Equipment.ExtraArm3)
	}
	if sourceChar.ExtraArms >= 4 && sourceChar.Equipment.ExtraArm4.ItemId > 0 && sourceChar.Equipment.ExtraArm4.GetSpec().Type == items.Weapon {
		attackWeapons = append(attackWeapons, sourceChar.Equipment.ExtraArm4)
	}

	// Empty hand slots become fist attacks (unless holding a shield or
	// blocked by a 2H weapon in the pair-partner slot — a 2H physically
	// occupies the partner arm even though its ItemId reads as 0).
	emptyArm := items.Item{ItemId: 0}

	// Main hand empty → fist. Weapon is always the first slot of pair A,
	// so it can never be blocked by a 2H.
	if sourceChar.Equipment.Weapon.ItemId == 0 {
		attackWeapons = append(attackWeapons, emptyArm)
	}
	// Offhand empty → fist, unless blocked by a 2H in the main hand.
	if sourceChar.Equipment.Offhand.ItemId == 0 &&
		!sourceChar.Equipment.IsBlockedBy2H("offhand") {
		attackWeapons = append(attackWeapons, emptyArm)
	}
	// Extra arm empty slots → fist. Even-numbered extras (2, 4) may be
	// blocked by a 2H in the paired odd-numbered slot (1, 3).
	if sourceChar.ExtraArms >= 1 && sourceChar.Equipment.ExtraArm1.ItemId == 0 {
		attackWeapons = append(attackWeapons, emptyArm)
	}
	if sourceChar.ExtraArms >= 2 && sourceChar.Equipment.ExtraArm2.ItemId == 0 &&
		!sourceChar.Equipment.IsBlockedBy2H("extra arm 2") {
		attackWeapons = append(attackWeapons, emptyArm)
	}
	if sourceChar.ExtraArms >= 3 && sourceChar.Equipment.ExtraArm3.ItemId == 0 {
		attackWeapons = append(attackWeapons, emptyArm)
	}
	if sourceChar.ExtraArms >= 4 && sourceChar.Equipment.ExtraArm4.ItemId == 0 &&
		!sourceChar.Equipment.IsBlockedBy2H("extra arm 4") {
		attackWeapons = append(attackWeapons, emptyArm)
	}

	// Fallback: must have at least one attack
	if len(attackWeapons) == 0 {
		attackWeapons = append(attackWeapons, items.Item{ItemId: 0})
	}

	return attackWeapons
}

// calcDualWieldPenalty computes the hit penalty for dual wielding.
func calcDualWieldPenalty(sourceChar *characters.Character, weapIdx, totalWeaps int) int {
	if totalWeaps <= 1 {
		return 0
	}

	dualWieldLevel := sourceChar.GetSkillLevel(skills.WeaponCombat)

	penalty := 0
	// Natural weapons (claws, fists, bare hands) ignore dual-wield penalty
	mainSub := sourceChar.Equipment.Weapon.GetSpec().Subtype
	offSub := sourceChar.Equipment.Offhand.GetSpec().Subtype
	mainIsNatural := mainSub == items.Claws || mainSub == items.Fist || sourceChar.Equipment.Weapon.ItemId == 0
	offIsNatural := offSub == items.Claws || offSub == items.Fist || sourceChar.Equipment.Offhand.ItemId == 0
	if mainIsNatural && offIsNatural {
		penalty = 0
	} else {
		penaltyReduction := float64(dualWieldLevel) / 50.0
		penalty = int(50.0 - (penaltyReduction * 40.0))
		if penalty < 10 {
			penalty = 10
		}
	}
	// Extra arm weapons get escalating penalties: +20 per arm beyond offhand
	if weapIdx >= 2 {
		penalty += (weapIdx - 1) * 20
	}
	return penalty
}

// buildWeaponSetup pre-computes weapon info for a single weapon in the attack sequence.
// Note: swing count is now computed separately by calcSwingCount, not here.
func buildWeaponSetup(sourceChar *characters.Character, targetChar *characters.Character, weapon items.Item, idx, total int) weaponSetup {
	bal := configs.GetBalanceConfig()
	// GetSpecies is a bare map lookup and returns nil for an unknown
	// SpeciesId, so the name must be defaulted before any dereference.
	// Matches the fallback used further down in this file.
	raceInfo := species.GetSpecies(sourceChar.SpeciesId)
	unarmedName := "fists"
	if raceInfo != nil {
		unarmedName = raceInfo.UnarmedName
	}
	ws := weaponSetup{
		weapon:        weapon,
		weaponName:    unarmedName,
		weaponSubType: items.Unarmed,
		weaponSpeed:   float64(bal.UnarmedSpeedMultiplier),
		isOffhand:     idx > 0,
		penalty:       calcDualWieldPenalty(sourceChar, idx, total),
	}

	ws.attacks, ws.baseDmg, ws.dmgVariance, ws.critBuffs = sourceChar.GetDefaultDistributionDamage()

	// Non-human basic attacks render through the species' natural-attack
	// subtype (bite/claws/slam/...) instead of generic. A real equipped weapon
	// overrides this below. Humanoids leave NaturalAttack empty and stay on
	// Unarmed -> generic.
	if raceInfo != nil && raceInfo.NaturalAttack != "" {
		ws.weaponSubType = raceInfo.NaturalAttack
	}

	if weapon.ItemId > 0 {
		itemSpec := weapon.GetSpec()
		ws.weaponName = weapon.DisplayName()
		ws.weaponSubType = itemSpec.Subtype
		ws.attacks, ws.baseDmg, ws.dmgVariance, ws.critBuffs = weapon.GetDistributionDamage()
		ws.weaponSpeed = itemSpec.GetSpeedMultiplier()

		// Racial bonus
		ws.baseDmg += float64(weapon.StatMod(string(statmods.RacialBonusPrefix) + strings.ToLower(targetChar.Species())))

		gearMul := mutations.GearEffectivenessMultiplier(sourceChar.Mutations)
		// T3 (chunk 4c): apply reach-utility penalty — long weapons are
		// penalized in grapple positions. CalcReachAdjustedItemMult uses
		// DamageMultiplier as the base and scales by radius/reach. The
		// resulting effective multiplier is then further scaled by gearMul.
		adjustedMult := CalcReachAdjustedItemMult(weapon, sourceChar)
		// Ranged (Shooting-subtype) weapons reserve their full damage multiplier
		// for the deliberate SHOOT path (actions.ExecuteFire). In the auto-attack
		// MELEE swing path a bow/crossbow/pistol is just an awkward improvised
		// club, so clamp the melee multiplier — otherwise a high-multiplier
		// arbalest would club like a god-maul.
		if itemSpec.Subtype == items.Shooting && adjustedMult > unloadedMeleeDamageCap {
			adjustedMult = unloadedMeleeDamageCap
		}
		ws.weaponDmgMult = adjustedMult * gearMul
		if ws.weaponDmgMult <= 0 {
			ws.weaponDmgMult = float64(bal.UnarmedDamageMultiplier)
		}
	} else {
		// Natural-attack path (mob unarmed, species DamageMultiplier).
		// Apply reach-utility directly: mob claws/bite/fist are short-reach
		// and stay effective in grapples, so this is mostly a correctness
		// guard — the numbers don't change for natural attacks in practice.
		naturalReach := items.ResolveNaturalReach(ws.weaponSubType)
		posRadius := 0.0 // default: no grapple penalty if Position not initialized
		if sourceChar.Position != nil {
			posRadius = PositionReachRadius(sourceChar.Position.State())
		}
		reachMult := ReachUtility(naturalReach, posRadius)
		if speciesInfo := species.GetSpecies(sourceChar.SpeciesId); speciesInfo != nil && speciesInfo.DamageMultiplier > 0 {
			ws.weaponDmgMult = speciesInfo.DamageMultiplier * reachMult
		} else {
			ws.weaponDmgMult = float64(bal.UnarmedDamageMultiplier) * reachMult
		}
	}

	return ws
}

// buildDamageParams computes the damage pipeline values for a weapon swing.
func buildDamageParams(sourceChar *characters.Character, targetChar *characters.Character, ws weaponSetup, statModBonus int, srcType SourceTarget) swingDamageParams {
	combatSkillLevel := sourceChar.GetCombatSkillLevel()
	rawDmg := CalcRawDamage(sourceChar.Stats.Strength.ValueAdj, combatSkillLevel, ws.weaponDmgMult, ChannelPhysical)

	// Apply mob damage multiplier
	if srcType == Mob {
		rawDmg *= float64(configs.GetBalanceConfig().MobDamageMultiplier)
	}

	// Apply target's physical mitigation
	dmgMean := ApplyMitigation(rawDmg, targetChar.GetPhysicalMitigation(), MitigationCap(ChannelPhysical))

	// Track pre-mitigation damage for crits
	rawDmgForCrit := rawDmg

	// Add statmod damage bonus
	dmgMean += float64(statModBonus)
	rawDmgForCrit += float64(statModBonus)

	// Apply smooth health-based melee damage penalty
	hpPenalty := float64(configs.GetBalanceConfig().HealthPenaltyMax)
	dmgMult := ResourceMultiplier(sourceChar.Health, sourceChar.HealthMax.Value, hpPenalty)
	dmgMean *= dmgMult
	rawDmgForCrit *= dmgMult

	// Stage 7.5: Apply prone damage penalty. Chunk 4b R1: FSM-driven —
	// Supine attackers swing just as poorly as Prone, so include both.
	if sourceChar.IsProne() || sourceChar.IsSupine() {
		dmgMean *= float64(configs.GetBalanceConfig().ProneDamagePenalty)
		rawDmgForCrit *= float64(configs.GetBalanceConfig().ProneDamagePenalty)
	}

	// Phase 24.2: Apply mutation damage multiplier
	if mutDmgMult := mutations.GetDamageMultiplier(sourceChar.Mutations); mutDmgMult != 0 {
		// #22 crash-site: inside the buried hull, mutation-driven power is
		// suppressed. GetDamageMultiplier returns the bonus fraction (applied
		// as 1.0+bonus), so dampen the full multiplier and re-extract the bonus
		// (penalties, i.e. multiplier <= 1.0, are left untouched by DampenBonus).
		if sourceChar.HasBuffFlag(buffs.Dampened) {
			factor := float64(configs.GetBalanceConfig().CrashSiteSuppressionFactor)
			mutDmgMult = mutations.DampenBonus(1.0+mutDmgMult, factor) - 1.0
		}
		dmgMean *= (1.0 + mutDmgMult)
		rawDmgForCrit *= (1.0 + mutDmgMult)
	}

	// Warcry condition: applies a physical damage multiplier from rhetoric shout
	if sourceChar.HasCondition(characters.ConditionWarcry) {
		warcryMult := 1.0 + sourceChar.GetConditionMagnitude(characters.ConditionWarcry)
		dmgMean *= warcryMult
		rawDmgForCrit *= warcryMult
	}

	// Message seed
	msgSeed := 0
	if configs.GetBalanceConfig().ConsistentAttackMessages {
		msgSeed = ws.weapon.ItemId
	}

	return swingDamageParams{
		dmgMean:       dmgMean,
		rawDmgForCrit: rawDmgForCrit,
		critDmgMult:   CritDamageMultiplier(combatSkillLevel),
		msgSeed:       msgSeed,
	}
}

// calcAttackScore computes the attack roll score with all modifiers.
func calcAttackScore(sourceChar *characters.Character, targetChar *characters.Character, penalty int, ctx combatContext) float64 {
	bal := configs.GetBalanceConfig()
	attackScore := float64(sourceChar.GetEffectiveDexterity()) + float64(sourceChar.GetCombatSkillLevel())*float64(bal.SkillWeight)
	attackScore -= float64(penalty)

	// Apply smooth stamina-based hit chance penalty
	spPenalty := float64(bal.StaminaPenaltyMax)
	staminaMult := ResourceMultiplier(sourceChar.Stamina, sourceChar.StaminaMax.Value, spPenalty)
	attackScore *= staminaMult

	// Stage 7.5: Apply prone attack multipliers. Chunk 4b R1: FSM-driven —
	// Supine maps to the same modifier as Prone (legacy enum couldn't
	// distinguish the two).
	if sourceChar.IsProne() || sourceChar.IsSupine() {
		attackScore *= float64(bal.ProneAttackMultiplier)
	}
	if targetChar.IsProne() || targetChar.IsSupine() {
		attackScore *= float64(bal.ProneVulnerabilityMultiplier)
	}

	// Chunk 5.11c: grapple position, moved here from calcCritThreshold so that
	// prone and grapple -- the same category of effect -- use the same
	// mechanism. Under margin-derived crit (5.11d) these also feed crit
	// automatically, because they move the margin.
	//
	// As threshold shifts these self-cancelled on the ground: BOTH participants
	// satisfy IsGroundGrapple() (a position state) while IsController() is a
	// separate control fsm, so the controller's -0.4 and the victim's +0.4 net
	// to zero. As multipliers they compound, which is the intent.
	if sourceChar.IsController() {
		if sourceChar.IsGroundGrapple() {
			attackScore *= float64(bal.GrappleGroundControlAttackMultiplier)
		} else if sourceChar.IsStandingGrapple() {
			attackScore *= float64(bal.GrappleStandingControlAttackMultiplier)
		}
	}
	if targetChar.IsGroundGrapple() && !targetChar.IsController() {
		attackScore *= float64(bal.GrappleGroundedVulnerabilityMultiplier)
	}

	// Darkness penalty: attacker can't see
	if !ctx.sourceCanSee {
		attackScore *= float64(bal.DarknessCombatPenalty)
	}

	// Winged Flight: a flyer beats the earthbound on the melee opposed roll —
	// striking from a superior angle (attacker flying) or staying out of an
	// earthbound reach (defender flying). Cancels flyer-vs-flyer / grounded-vs-
	// grounded. Applied additively to the attacker's score.
	if edge := flightEdge(mutations.IsFlying(sourceChar.Mutations), mutations.IsFlying(targetChar.Mutations),
		int(bal.FlightOpposedEdge)); edge != 0 {
		attackScore += float64(edge)
	}

	return attackScore
}

// calcCritThreshold computes the z-score threshold for critical hits.
func calcCritThreshold(sourceChar *characters.Character, targetChar *characters.Character) float64 {
	critThreshold := 2.0
	if sourceChar.HasBuffFlag(buffs.Accuracy) {
		critThreshold = 1.5
	}
	if targetChar.HasBuffFlag(buffs.Blink) {
		critThreshold = 2.5
	}
	// Skill advantage shifts the crit threshold.
	//
	// READ THIS BEFORE REASONING ABOUT CRIT RATES. What this function returns
	// is only the BAR. Since chunk 5.11d the thing measured against that bar is
	// the normalized opposed-roll MARGIN, not a self-relative z-score, so the
	// opponent's stats, gear and position dominate the actual crit rate. See
	// margin_crit.go. A high-stat boss rolls high defences, the margin collapses
	// or goes negative, and crits become rare no matter what this returns; a
	// trash mob leaves an enormous margin and crits land constantly. Do NOT
	// conclude from the threshold alone that crit rate is matchup-independent —
	// that was the pre-5.11d behaviour and it is exactly what 5.11d removed.
	//
	// The skill term below DOES saturate, and that part is INTENDED. Confirmed
	// by playtest 2026-08-14 and kept deliberately: do not "fix" it.
	// GetCombatSkillLevel returns 1 when unset (characters/skills.go) and no mob
	// YAML defines a skills block, so every mob in the world is combat skill 1.
	// skillDiff is therefore the player's combat skill minus one, and at 0.05
	// per point the 1.5 floor is reached at combat skill 11. For nearly every
	// established character the bar is thus pinned at 1.5 rather than 2.0
	// against every target.
	//
	// At PARITY that is the difference between a 2.3% and a 6.7% crit rate
	// (ContestCritThreshold is 2.0 precisely to reproduce the legacy rate at
	// parity). Away from parity the margin, not the bar, is what decides.
	//
	// Note the double count, deliberate but worth knowing: combat skill already
	// raises the margin through the attack score (SkillWeight), and it lowers
	// this bar as well. Skill therefore reaches crit rate twice. The playtest
	// says this feels good, so it stays; rebalancing it changes FEEL, not a
	// defect.
	//
	// If mobs ever gain real combat skills, revisit: the slope was written for
	// two-sided values and would behave very differently.
	skillDiff := sourceChar.GetCombatSkillLevel() - targetChar.GetCombatSkillLevel()
	critThreshold -= float64(skillDiff) * 0.05

	// Floor after skill adjustment: never easier than Accuracy buff level (~6.7% crit)
	if critThreshold < 1.5 {
		critThreshold = 1.5
	}

	// Chunk 5.11c: position-based crit modifiers MOVED OUT to calcAttackScore,
	// alongside the prone multipliers that already lived there. Do not
	// reintroduce them here.
	//
	// Two reasons. First, prone and grapple are the same category of effect and
	// were using two different mechanisms in two different files. Second, the
	// ground-grapple pair silently cancelled: both participants satisfy
	// IsGroundGrapple() (a position state) while IsController() is a separate
	// control fsm, so the controller's -0.4 and the victim's +0.4 netted to
	// ZERO -- ground control, the stronger position, was worth strictly less
	// than the standing -0.2.
	//
	// Buff modifiers (Accuracy, Blink) stay on the threshold. They are not
	// positional; "this character crits more readily" is exactly what a
	// threshold expresses.

	// Absolute floor. Retained for the buff modifiers above.
	if critThreshold < 1.0 {
		critThreshold = 1.0
	}

	return critThreshold
}

// filterDefensesForThirdParty removes active defenses when the target is in a grapple
// and being attacked by a third party.
func filterDefensesForThirdParty(result *AttackResult, sourceChar *characters.Character, targetChar *characters.Character, defSeq []string) ([]string, bool) {
	isThirdParty := IsThirdPartyAttack(sourceChar, targetChar)
	if !isThirdParty {
		return defSeq, false
	}

	filteredDefenses := []string{}
	for _, def := range defSeq {
		if def == characters.DefenseBlock {
			filteredDefenses = append(filteredDefenses, def)
		}
	}

	// If no defenses remain, send vulnerability messages and auto-hit.
	// Vulnerability prose is hit-prep — the swing is about to land.
	if len(filteredDefenses) == 0 {
		hitCat := messaging.CategoryHitMelee
		if sourceChar.Equipment.Weapon.ItemId > 0 {
			hitCat = CategoryForWeaponSubtype(sourceChar.Equipment.Weapon.GetSpec().Subtype)
		}
		result.SendToTarget(hitCat, fmt.Sprintf(
			`<ansi fg="red">You're too entangled to defend against %s's attack!</ansi>`,
			sourceChar.Name))
		result.SendToSource(hitCat, fmt.Sprintf(
			`<ansi fg="attack-good">%s is helpless against your attack!</ansi>`,
			targetChar.Name))
		result.SendToSourceRoom(hitCat, fmt.Sprintf(
			`<ansi fg="combat">%s is defenseless against %s's attack!</ansi>`,
			targetChar.Name, sourceChar.Name))
	}

	return filteredDefenses, true
}

// runBestOfAllDefense rolls every available defense and picks the one that won
// by the widest margin. Returns the best result.
//
// Chunk U1: the rolling and selecting now live in internal/contest. Everything
// this function still does is melee-specific and deliberately stayed here —
// building each defence's score, tracking attempts for stance, and charging
// the winner.
//
// Chunk U5b-2 removed the affordability check that used to sit alongside those.
// Every defence in defSeq now enters the contest regardless of the defender's
// stamina, and only the winner is charged, partially. See the comment at the
// top of the entry loop for why.
func runBestOfAllDefense(result *AttackResult, sourceChar *characters.Character, targetChar *characters.Character, defSeq []string, atkScore float64, isThirdParty bool, ctx combatContext) bestDefenseResult {
	bal := configs.GetBalanceConfig()

	entries := make([]contest.Entry, 0, len(defSeq))

	for _, defenseType := range defSeq {
		// Track defense attempt
		result.DefenseAttempts = append(result.DefenseAttempts, DefenseType(defenseType))

		// Stage 9.4: Track defense for stance calculation
		targetChar.IncrementDefenseCount()

		// U5b-2: there is deliberately NO affordability gate here.
		//
		// This used to `continue` an unaffordable defence out of the candidate
		// set. With every defence unaffordable the entry list came out empty,
		// the contest reported uncontested, and the swing fell through to the
		// old MinDefenseChance last resort -- a flat 15% save, always narrated
		// as a dodge, and never able to defence-crit. U6 deleted that knob and
		// its narrator; an uncontested swing is now simply an attack win, and
		// the floor lives inside RunContest. An exhausted actor still acts; the
		// winning defence is charged partially below.
		//
		// The defender's exhaustion currently costs their defence NOTHING:
		// GetDefenseScore has no resource term, and stripping the skill term is
		// U8. That gap is deliberate and disclosed, not an oversight.

		// Calculate defense score for this defense type
		defenseScore := targetChar.GetDefenseScore(defenseType)

		// Apply base effectiveness multipliers
		switch defenseType {
		case characters.DefenseDodge:
			defenseScore *= float64(bal.DodgeEffectiveness)
		case characters.DefenseParry:
			defenseScore *= float64(bal.ParryEffectiveness)
		case characters.DefenseBlock:
			defenseScore *= float64(bal.BlockEffectiveness)
		}

		// Stage 7.5: Apply position-based defense penalties. Chunk 4b R1:
		// FSM-driven — Prone/Supine collapse to the legacy "prone"
		// penalty bucket, IsStandingGrapple matches the legacy
		// "clinched" bucket, IsGroundGrapple matches the legacy
		// "grounded" bucket.
		switch {
		case targetChar.IsProne() || targetChar.IsSupine():
			switch defenseType {
			case "dodge":
				defenseScore *= float64(bal.ProneDodgePenalty)
			case "parry":
				defenseScore *= float64(bal.ProneParryPenalty)
			case "block":
				defenseScore *= float64(bal.ProneBlockPenalty)
			}
		case targetChar.IsStandingGrapple():
			switch defenseType {
			case "dodge":
				defenseScore *= float64(bal.ClinchDodgePenalty)
			case "parry":
				defenseScore *= float64(bal.ClinchParryPenalty)
			case "block":
				defenseScore *= float64(bal.ClinchBlockPenalty)
			}
		case targetChar.IsGroundGrapple():
			switch defenseType {
			case "dodge":
				defenseScore *= float64(bal.GroundedDodgePenalty)
			case "parry":
				defenseScore *= float64(bal.GroundedParryPenalty)
			case "block":
				defenseScore *= float64(bal.GroundedBlockPenalty)
			}
		}

		// Rally condition: applies a defense score multiplier from rhetoric shout
		if targetChar.HasCondition(characters.ConditionRally) {
			defenseScore *= 1.0 + targetChar.GetConditionMagnitude(characters.ConditionRally)
		}

		// Stage 8.5: Apply third-party vulnerability penalty
		if isThirdParty {
			defenseScore *= float64(bal.ThirdPartyGrapplePenalty)
		}

		// Stage 8.6: Apply failed grapple defense penalty
		if targetChar.HasCondition(characters.ConditionDefensePenalty) {
			defenseScore *= targetChar.GetConditionMagnitude(characters.ConditionDefensePenalty)
		}

		// Darkness penalty: defender can't see
		if !ctx.targetCanSee {
			defenseScore *= float64(bal.DarknessCombatPenalty)
		}

		// Incorporeal mutation: physical defense bonus (channel-scoped
		// to physical attacks; this function only handles physical
		// swings — spells use a different resolution path).
		defenseScore += mutations.GetPhysicalDefenseBonus(targetChar.Mutations)

		entries = append(entries, contest.Entry{Name: defenseType, Score: defenseScore})
	}

	res := RunContest(atkScore, entries)

	best := bestDefenseResult{
		hitRoll: res.AttackRoll,
	}
	if res.Contested {
		best.defenseType = res.Winner
		best.defRoll = res.DefenseRoll
		best.floored = res.Floored
		// SIGN CONVERSION, and the only one in melee. contest.Result.Margin is
		// ATTACK-positive; bestDefenseResult.margin is DEFENCE-positive. Negate
		// exactly here and nowhere else. U6 deletes bestDefenseResult and this
		// conversion with it.
		best.margin = -res.Margin
	} else {
		// Preserve the legacy sentinel exactly. normalizedAttackMargin detects
		// "no defence attempted" via defenseType == "" and never reads this
		// value, but resolveDefenseOutcomeCore's `best.margin > 0` check does,
		// and -Inf is what makes an uncontested swing fall through to a hit.
		best.margin = math.Inf(-1)
	}

	// Charge stamina only for the winning defence.
	//
	// U5b-2: partial, not full-or-refuse. With the affordability gate above
	// removed, an exhausted defender can now win the contest, so this call must
	// be able to charge what little is there rather than declining and leaving
	// the defence free. U8 reads CostResult.Short to strip the skill term from
	// the defence score; this chunk discards it.
	if best.defenseType != "" {
		_ = targetChar.ApplyCostPartial(characters.PoolStamina,
			targetChar.GetDefenseStaminaCost(best.defenseType))
	}

	return best
}

// hitResolution holds the outcome of the full hitroll pipeline.
type hitResolution struct {
	hit          bool
	crit         bool
	fumble       bool
	doubleFumble bool
	defenseCrit  bool
	hitRoll      dice.RollResult
}

// doubleFumbleMessages are comedy flavor text for when both combatants fumble.
var doubleFumbleMessages = []struct {
	toAttacker string
	toDefender string
	toRoom     string
}{
	{
		toAttacker: `You trip over your own feet and %s stumbles trying to capitalize!`,
		toDefender: `%s trips over their own feet and you stumble trying to capitalize!`,
		toRoom:     `%s and %s both stumble in a spectacular display of ineptitude.`,
	},
	{
		toAttacker: `You swing wildly and lose your balance — %s flails just as badly!`,
		toDefender: `%s swings wildly and loses balance — and you flail just as badly!`,
		toRoom:     `%s and %s both flail about in an embarrassing tangle of limbs.`,
	},
	{
		toAttacker: `Your weapon slips at the exact moment %s trips over their own guard!`,
		toDefender: `Your guard tangles at the exact moment %s's weapon slips free!`,
		toRoom:     `%s's weapon slips and %s's guard tangles — both stumble to the ground!`,
	},
	{
		toAttacker: `You overcommit and tumble forward — %s overreacts and falls too!`,
		toDefender: `%s overcommits and tumbles forward — you overreact and fall too!`,
		toRoom:     `%s overcommits and %s overreacts — both crash to the ground!`,
	},
	{
		toAttacker: `You slip on something and %s panics into a heap beside you!`,
		toDefender: `%s slips on something and you panic into a heap beside them!`,
		toRoom:     `%s slips and %s panics — both end up in a heap on the ground!`,
	},
}

// handleDoubleFumble applies prone to both combatants and sends comedy text.
func handleDoubleFumble(result *AttackResult, sourceChar *characters.Character, targetChar *characters.Character) {
	// Both go prone via FSM. Position can be nil on a pre-Validate()
	// Character (mirrors the guard in HandleGrappleCritFailure); prod
	// combatants are always Validated, so this only shields the
	// under-initialized-fixture case rather than any live path.
	r := state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward}
	if sourceChar.Position != nil {
		if err := sourceChar.Position.TransitionToProne(position.ProneData{}, r); err != nil {
			mudlog.Warn("handleDoubleFumble: source TransitionToProne failed", "err", err)
		}
	}
	if targetChar.Position != nil {
		if err := targetChar.Position.TransitionToProne(position.ProneData{}, r); err != nil {
			mudlog.Warn("handleDoubleFumble: target TransitionToProne failed", "err", err)
		}
	}

	// Pick a random comedy message
	msg := doubleFumbleMessages[util.Rand(len(doubleFumbleMessages))]

	// Double-fumble: both sides fumble, no defense, no clean
	// weapon category to pick. Use CategoryHitMelee as the combat-
	// neutral hit-band default.
	fumbleCat := messaging.CategoryHitMelee
	result.SendToSource(fumbleCat, fmt.Sprintf(`<ansi fg="fumble-text">!!!</ansi> `+
		`<ansi fg="yellow">`+msg.toAttacker+`</ansi>`+
		` <ansi fg="fumble-text">!!!</ansi>`, targetChar.Name))
	result.SendToTarget(fumbleCat, fmt.Sprintf(`<ansi fg="fumble-text">!!!</ansi> `+
		`<ansi fg="yellow">`+msg.toDefender+`</ansi>`+
		` <ansi fg="fumble-text">!!!</ansi>`, sourceChar.Name))
	result.SendToSourceRoom(fumbleCat, fmt.Sprintf(`<ansi fg="fumble-text">!!!</ansi> `+
		`<ansi fg="yellow">`+msg.toRoom+`</ansi>`+
		` <ansi fg="fumble-text">!!!</ansi>`, sourceChar.Name, targetChar.Name))
}

// resolveDefenseOutcome processes the best defense result. Priority is
// fumbles → winner (already floored by RunContest) → crit → normal outcome.
// Returns the full hitResolution including crit/fumble flags.
//
// U6 Task 8 moved the floor AHEAD of crit. It used to be the last step, after
// five crit branches that all returned, so against a defender who reliably
// defence-crit the attack floor was evaluated on almost nothing. The contest
// floor now decides the winner inside RunContest and crit is derived from that
// settled winner, which is why MinAttackHitChance and MinDefenseChance are gone
// rather than merely relocated: keeping them would have been a second floor
// layered on the first, each partly undoing the other.
//
// forceCrit bypasses the crit check entirely and treats the attack as a
// confirmed crit regardless of the roll, additionally suppressing the fumble
// branch so a forced crit cannot resolve as a fumble. Pass false for all normal
// combat rounds; T14 passes true for sleeping-victim first-hit-crit.
//
// Chunk 5.11d: this used to work by writing critThreshold+0.5 into
// best.hitRoll.ZScore. Crit no longer reads that field, so the mutation was
// removed rather than left as a silent no-op. One visible consequence:
// result.AttackZScore now reports the roll that actually happened instead of the
// bumped value.
func resolveDefenseOutcome(result *AttackResult, best bestDefenseResult, sourceChar *characters.Character, targetChar *characters.Character, critThreshold float64, isThirdParty bool, forceCrit bool) hitResolution {
	res := resolveDefenseOutcomeCore(result, best, sourceChar, targetChar, critThreshold, isThirdParty, forceCrit)

	// Chunk 5.11e: crit floors run HERE, after every branch above has settled
	// res.hit, and nowhere earlier. The core resolver treats an attack crit as
	// forcing a hit, so a floor evaluated inside it would become an undeclared
	// second hit floor. That used to leak through MinDefenseChance; U6 deleted
	// that knob, and the contest floor now lives inside RunContest, but the
	// one-application-point rule still holds. See applyCritFloors.
	applyCritFloors(&res, result, best, AttackCritFloor(), DefenseCritFloor())

	return res
}

// resolveDefenseOutcomeCore is resolveDefenseOutcome without the crit floors.
// Split out so the floors have exactly one application point despite the many
// early returns below.
func resolveDefenseOutcomeCore(result *AttackResult, best bestDefenseResult, sourceChar *characters.Character, targetChar *characters.Character, critThreshold float64, isThirdParty bool, forceCrit bool) hitResolution {
	fumbleThreshold := -2.0
	defCritThreshold := 2.0

	res := hitResolution{
		hitRoll: best.hitRoll,
	}

	// Store z-scores from the best defense attempt
	if best.defenseType != "" {
		result.AttackZScore = best.hitRoll.ZScore
		result.DefenseZScore = best.defRoll.ZScore
	}

	// Fumbles deliberately REMAIN on the self-relative z-score. They share the
	// architectural quirk crits had, but moving them would change failure rates
	// nobody asked to change. Explicitly out of scope for chunk 5.11d — do not
	// "fix" them in passing.
	attackFumble := best.hitRoll.ZScore <= fumbleThreshold
	defenseFumble := best.defenseType != "" && best.defRoll.ZScore <= fumbleThreshold

	// Chunk 5.11d: crit derives from the normalized opposed-roll margin, so
	// winning decisively is what makes you crit. See margin_crit.go for the
	// sign, infinity and normaliser traps.
	var attackCrit bool
	if z, ok := normalizedAttackMargin(best); ok {
		attackCrit = z >= critThreshold
	} else {
		// T2: no defence was attempted, so there is no contest and no margin to
		// derive from. Fall back to the legacy self-relative check, which
		// preserves prior behaviour exactly on that path. Do NOT synthesise a
		// margin from math.Inf(-1).
		attackCrit = best.hitRoll.ZScore >= critThreshold
	}

	// Mirror of the attack side, and it must mirror it: an earlier draft left
	// the defence with no fallback, so whenever the margin was unusable defence
	// crit silently became impossible rather than falling back. "No defence
	// attempted" is the only case that legitimately yields false.
	var defenseCrit bool
	if best.defenseType != "" {
		if z, ok := normalizedDefenseMargin(best); ok {
			defenseCrit = z >= defCritThreshold
		} else {
			defenseCrit = best.defRoll.ZScore >= defCritThreshold
		}
	}

	// T4: forceCrit (the sleeping-victim first hit) is now an explicit flag.
	// It previously worked by writing critThreshold+0.5 into
	// best.hitRoll.ZScore — a field crit no longer reads, so that form would
	// have become a silent no-op.
	//
	// That bump also lifted the roll clear of the fumble threshold, and the
	// fumble branch runs FIRST and returns. Clearing attackFumble here preserves
	// that behaviour; without it a forced crit on a terrible roll would resolve
	// as a fumble instead.
	if forceCrit {
		attackCrit = true
		attackFumble = false
	}

	// ── Step 1: Fumble resolution (absolute) ────────────────────────────────

	if attackFumble && defenseFumble {
		// Double fumble: miss, both go prone, comedy text
		res.fumble = true
		res.doubleFumble = true
		res.hit = false
		result.Fumble = true
		result.DoubleFumble = true
		handleDoubleFumble(result, sourceChar, targetChar)
		mudlog.Debug("DoubleFumble", "atkZ", fmt.Sprintf("%.2f", best.hitRoll.ZScore),
			"defZ", fmt.Sprintf("%.2f", best.defRoll.ZScore),
			"source", sourceChar.Name, "target", targetChar.Name)
		return res
	}

	if attackFumble {
		// Attack fumble: always miss, no exceptions
		res.fumble = true
		res.hit = false
		result.Fumble = true
		mudlog.Debug("AttackFumble", "zScore", fmt.Sprintf("%.2f", best.hitRoll.ZScore),
			"source", sourceChar.Name, "target", targetChar.Name)
		return res
	}

	if defenseFumble {
		// Defense fumble: guarantees a hit (but NOT auto-crit)
		res.hit = true
		mudlog.Debug("DefenseFumble", "defZ", fmt.Sprintf("%.2f", best.defRoll.ZScore),
			"source", sourceChar.Name, "target", targetChar.Name)
		// Still check if the attack roll was also a crit
		if attackCrit {
			res.crit = true
		}
		return res
	}

	// ── Step 2: the floor already gated the winner, inside RunContest ───────
	//
	// This used to sit AFTER crit resolution, which returned on five branches,
	// so against a defender who reliably defence-crit the attack floor was
	// evaluated on almost nothing. The Core Guardian defence-crit 96.8% of
	// swings and the floor saw the other 3.2%.
	//
	// RunContest has ALREADY flipped the winner and stamped the +-1 sentinel by
	// the time we get here, so best.margin is post-flip and this one expression
	// covers both the floored and unfloored cases. A floored hit carries
	// res.Margin +1, which the sign conversion turns into best.margin -1, so it
	// reads as an attack win; a floored save is the mirror. Do not special-case
	// best.floored here -- it matters only for the crit gate below.
	//
	// An UNCONTESTED swing leaves best.margin at math.Inf(-1) and is likewise an
	// attack win, which is the legacy behaviour of the `best.margin > 0` test
	// this replaces.
	attackWon := best.margin <= 0

	// forceCrit is a decision taken BEFORE the roll (the sleeping-victim first
	// hit), not a crit derived from winning. Now that crit is gated on the
	// winning side it has to force the win too: without this, a sleeper whose
	// defence happened to take the margin would quietly resolve as an ordinary
	// miss and the documented "first round against a sleeper auto-crits"
	// contract would break on roughly half of swings, silently.
	if forceCrit {
		attackWon = true
	}

	// ── Step 3: crit, only on the side that WON, and never when floored ─────
	//
	// A floored outcome carries the sentinel margin rather than a real one.
	// Promoting it would hand a decisive result to the side that lost the roll.
	// The sentinel normalises to a near-zero z, so this gate is belt-and-braces
	// today -- but it is the DECLARED rule, and it stops a future retune of the
	// sentinel from silently reintroducing floored crits.
	//
	// forceCrit is exempt for the same reason it forces the win: it is not
	// derived from the margin at all, so the sentinel says nothing about it.
	if forceCrit || !best.floored {
		if attackWon && attackCrit {
			res.hit = true
			res.crit = true
			mudlog.Debug("AttackCrit", "zScore", fmt.Sprintf("%.2f", best.hitRoll.ZScore),
				"threshold", fmt.Sprintf("%.2f", critThreshold),
				"source", sourceChar.Name, "target", targetChar.Name)
			return res
		}
		if !attackWon && defenseCrit {
			res.hit = false
			res.defenseCrit = true
			setDefenseCritFlags(result, best)
			sendDefenseMessages(result, best, sourceChar, targetChar, isThirdParty)
			mudlog.Debug("DefenseCrit", "defZ", fmt.Sprintf("%.2f", best.defRoll.ZScore),
				"source", sourceChar.Name, "target", targetChar.Name)
			return res
		}
	}

	// ── Step 4: normal outcome ──────────────────────────────────────────────

	if attackWon {
		res.hit = true
		return res
	}

	res.hit = false
	sendDefenseMessages(result, best, sourceChar, targetChar, isThirdParty)
	return res
}

// setDefenseCritFlags marks parry/dodge/block crit flags on the result.
func setDefenseCritFlags(result *AttackResult, best bestDefenseResult) {
	switch best.defenseType {
	case characters.DefenseParry:
		result.ParryCritDetected = true
	case characters.DefenseDodge:
		result.DodgeCritDetected = true
	case characters.DefenseBlock:
		result.BlockCritDetected = true
	}
}

// sendDefenseMessages sends narrative messages for a successful defense.
func sendDefenseMessages(result *AttackResult, best bestDefenseResult, sourceChar *characters.Character, targetChar *characters.Character, isThirdParty bool) {
	result.DefenseUsed = DefenseType(best.defenseType)

	var defenseVerb string
	var skillToProgress string
	var itemsDefenseType items.DefenseType
	switch best.defenseType {
	case characters.DefenseDodge:
		defenseVerb = "dodge"
		itemsDefenseType = items.DefenseDodge
		skillToProgress = string(skills.UnarmedCombat)
	case characters.DefenseParry:
		defenseVerb = "parry"
		itemsDefenseType = items.DefenseParry
		skillToProgress = string(skills.WeaponCombat)
	case characters.DefenseBlock:
		defenseVerb = "block"
		itemsDefenseType = items.DefenseBlock
		skillToProgress = string(skills.WeaponCombat)
	}

	// Trigger skill progression for successful defense
	targetChar.TrackSkillUse(skillToProgress)
	targetChar.CheckSkillProgression(skillToProgress, targetChar.GetUserId(), 1.0)

	// Get narrative defense messages based on defense z-score
	defenseMsgs := items.GetDefenseMessage(itemsDefenseType, best.defRoll.ZScore)

	// Prepare token replacements
	weaponName := "fists"
	attackName := "strike" // Generic term for unarmed attacks
	if raceInfo := species.GetSpecies(sourceChar.SpeciesId); raceInfo != nil {
		weaponName = raceInfo.UnarmedName
	}
	if sourceChar.Equipment.Weapon.ItemId > 0 {
		weaponName = sourceChar.Equipment.Weapon.GetSpec().Name
		attackName = weaponName
	}

	tokenReplacements := map[items.TokenName]string{
		items.TokenDefender: targetChar.Name,
		items.TokenAttacker: sourceChar.Name,
		items.TokenWeapon:   weaponName,
		items.TokenAttack:   attackName,
		items.TokenStance:   targetChar.CalculateStanceString(),
		items.TokenPosition: targetChar.CalculatePositionString(),
		items.TokenMomentum: targetChar.CalculateMomentumString(),
	}

	// If we have custom defense messages, use them
	if len(defenseMsgs.Together.ToDefender) > 0 {
		toDefenderMsg := defenseMsgs.Together.ToDefender.Get()
		toAttackerMsg := defenseMsgs.Together.ToAttacker.Get()
		toRoomMsg := defenseMsgs.Together.ToRoom.Get()

		for token, value := range tokenReplacements {
			toDefenderMsg = toDefenderMsg.SetTokenValue(token, value)
			toAttackerMsg = toAttackerMsg.SetTokenValue(token, value)
			toRoomMsg = toRoomMsg.SetTokenValue(token, value)
		}

		defCat := CategoryForDefenseVerb(defenseVerb)
		result.SendToTarget(defCat, string(toDefenderMsg))
		result.SendToSource(defCat, string(toAttackerMsg))
		result.SendToSourceRoom(defCat, string(toRoomMsg))
		if sourceChar.RoomId != targetChar.RoomId {
			result.SendToTargetRoom(defCat, string(toRoomMsg))
		}
	} else {
		defCat := CategoryForDefenseVerb(defenseVerb)
		result.SendToSource(defCat, fmt.Sprintf(`<ansi fg="attack-bad">%s %ss your attack!</ansi>`, targetChar.Name, defenseVerb))
		result.SendToTarget(defCat, fmt.Sprintf(`<ansi fg="defense-good">You %s %s's attack!</ansi>`, defenseVerb, sourceChar.Name))
		result.SendToSourceRoom(defCat, fmt.Sprintf(`<ansi fg="combat">%s %ss %s's attack.</ansi>`, targetChar.Name, defenseVerb, sourceChar.Name))
		if sourceChar.RoomId != targetChar.RoomId {
			result.SendToTargetRoom(defCat, fmt.Sprintf(`<ansi fg="combat">%s %ss an attack.</ansi>`, targetChar.Name, defenseVerb))
		}
	}

	// Stage 8.5: Add third-party context if applicable
	if isThirdParty {
		result.SendToTarget(CategoryForDefenseVerb(defenseVerb), fmt.Sprintf(
			`<ansi fg="yellow">(Despite being entangled in a grapple!)</ansi>`))
	}
}

// U6 Task 8 deleted sendFloorDefenseMessages. It narrated the MinDefenseChance
// last-resort save, which always claimed a dodge because that path had no real
// winning defence to name. A floored save now comes out of the contest with a
// genuine best.defenseType and is narrated by sendDefenseMessages like any other
// successful defence, so the player is told what actually stopped the swing.

// calcHitDamage computes the damage for a successful hit, handling crits.
// The isCrit flag is determined during hitroll resolution, not re-derived here.
func calcHitDamage(result *AttackResult, isCrit bool, backstab bool, sdp swingDamageParams) (int, bool) {
	if isCrit || backstab {
		result.Crit = true
		result.BuffTarget = sdp.critBuffs
		// Crits bypass mitigation, so they roll around the UNmitigated mean —
		// and therefore must take their spread from that same mean. RollStat
		// derives stdDev = mean * RollSpread internally, which is the only way
		// to keep the two in step.
		//
		// Chunk 5.11g: the skill-scaled crit multiplier is applied to the MEAN,
		// before the roll, for that same reason — scaling the rolled result
		// instead would stretch the spread by the multiplier and leave crits
		// wildly swingier at high skill.
		critMean := sdp.rawDmgForCrit * sdp.critDmgMult
		damageResult := dice.RollStat(critMean)
		dmg := int(math.Round(math.Max(0, damageResult.Value)))
		mudlog.Debug("CritDamage", "rawDmg", fmt.Sprintf("%.1f", sdp.rawDmgForCrit), "critMult", fmt.Sprintf("%.2f", sdp.critDmgMult), "critMean", fmt.Sprintf("%.1f", critMean), "mitigatedDmg", fmt.Sprintf("%.1f", sdp.dmgMean))
		return dmg, false // consume backstab
	}
	// Normal hit: use mitigated damage
	damageResult := dice.RollStat(sdp.dmgMean)
	return int(math.Round(math.Max(0, damageResult.Value))), backstab
}

// swingDamageParamsWithCritBuffs is a type alias to carry critBuffs through calcHitDamage
// critBuffs are stored via sdp so they pass through naturally.

// meleeDisplaySubtype computes the subtype used to select AUTO-ATTACK melee
// swing narration. Two rules apply, in order:
//
//  1. Shooting-subtype weapons (bows/crossbows/slings/guns) ALWAYS narrate as
//     improvised Bludgeoning in the melee path — a swung bow is a club, never
//     "fires/snipes/arrow". The deliberate SHOOT path (actions.ExecuteFire /
//     combat_fire.go) has its own messages and never routes through here, so
//     this is unconditional regardless of grapple position.
//  2. Otherwise, when ShouldBludgeon fires (weapon reach exceeds the grapple
//     radius of the attacker's position), bladed subtypes (Slashing, Cleaving,
//     Stabbing) narrate as Bludgeoning — the fiction tracks the pommel/hilt
//     strike that the reach damage penalty already reflects.
//
// Natural-blunt subtypes (Fist, Claws, Bite, Sting, Slam, Gore, Whipping) and
// caster subtypes (Wand, Sceptre, Staff) keep their own vocabulary.
func meleeDisplaySubtype(weaponSubType items.ItemSubType, weaponReach, posRadius float64) items.ItemSubType {
	if weaponSubType == items.Shooting {
		return items.Bludgeoning
	}
	if ShouldBludgeon(weaponReach, posRadius) {
		switch weaponSubType {
		case items.Slashing, items.Cleaving, items.Stabbing:
			return items.Bludgeoning
		}
	}
	return weaponSubType
}

// buildAttackMessages constructs and sends all combat messages for a swing.
func buildAttackMessages(result *AttackResult, sourceChar *characters.Character, targetChar *characters.Character,
	ws weaponSetup, sdp swingDamageParams, attackTargetDamage int, attackTargetReduction int,
	attackSourceDamage int, attackSourceReduction int,
	srcType, tgtType SourceTarget, prefix string) {

	// Calculate actual damage vs. expected damage pct
	pctDamage := 0.0
	if sdp.dmgMean > 0 {
		pctDamage = math.Ceil(float64(attackTargetDamage) / sdp.dmgMean * 100)
	}

	// T4 (chunk 4c): compute the display subtype for attack-message selection.
	// See meleeDisplaySubtype for the swap rules.
	displaySubtype := ws.weaponSubType
	{
		var weaponReach float64
		if ws.weapon.ItemId > 0 {
			spec := ws.weapon.GetSpec()
			weaponReach = items.ResolveReach(&spec)
		} else {
			weaponReach = items.ResolveNaturalReach(ws.weaponSubType)
		}
		posRadius := 0.0
		if sourceChar.Position != nil {
			posRadius = PositionReachRadius(sourceChar.Position.State())
		}
		displaySubtype = meleeDisplaySubtype(ws.weaponSubType, weaponReach, posRadius)
	}

	// Use fumble messages when a fumble is detected
	var msgs items.AttackOptions
	isFeint := false
	if result.Fumble {
		msgs = items.GetPreAttackMessage(displaySubtype, items.Fumble)
	} else if IsDying(targetChar) {
		// U5c: the target has already taken its lethal blow and the attributed
		// death is queued. Later hits this round still connect and still count
		// toward the damage map, but they read as finishing an opponent rather
		// than as ordinary swings, and never as a second kill announcement --
		// the killing blow is announced exactly once, by whoever landed it.
		//
		// After the fumble branch on purpose: flubbing a swing at a falling
		// target is still a fumble.
		msgs = items.GetPreAttackMessage(displaySubtype, items.CoupDeGrace)
	} else {
		msgs = items.GetAttackMessage(displaySubtype, int(pctDamage))
		// Feint check: skilled attackers can turn misses into deliberate-looking feints
		if int(pctDamage) == 0 && !result.Fumble {
			isFeint = checkFeint(sourceChar.GetCombatSkillLevel())
		}
	}

	var toAttackerMsg, toDefenderMsg, toAttackerRoomMsg, toDefenderRoomMsg items.ItemMessage

	tokenReplacements := map[items.TokenName]string{
		items.TokenItemName:     ws.weaponName,
		items.TokenSource:       sourceChar.Name,
		items.TokenSourceType:   string(srcType) + `name`,
		items.TokenTarget:       targetChar.Name,
		items.TokenTargetType:   string(tgtType) + `name`,
		items.TokenUsesLeft:     `[Invalid]`,
		items.TokenDamage:       GetDamageDescription(attackTargetDamage, targetChar.HealthMax.Value),
		items.TokenEntranceName: `unknown`,
		items.TokenExitName:     `unknown`,
		items.TokenStance:       sourceChar.CalculateStanceString(),
		items.TokenPosition:     sourceChar.CalculatePositionString(),
		items.TokenMomentum:     sourceChar.CalculateMomentumString(),
		items.TokenBodyPart:     GetRandomBodyPart(),
	}

	// Get source character's weapon skill level for message selection
	skillLevel := sourceChar.GetCombatSkillLevel()

	if sourceChar.RoomId == targetChar.RoomId {
		toAttackerMsg = msgs.Together.ToAttacker.GetForSkillLevel(skillLevel, sdp.msgSeed)
		toDefenderMsg = msgs.Together.ToDefender.GetForSkillLevel(skillLevel, sdp.msgSeed)
		toAttackerRoomMsg = msgs.Together.ToRoom.GetForSkillLevel(skillLevel, sdp.msgSeed)
		toDefenderRoomMsg = items.ItemMessage("")
	} else {
		toAttackerMsg = msgs.Separate.ToAttacker.GetForSkillLevel(skillLevel, sdp.msgSeed)
		toDefenderMsg = msgs.Separate.ToDefender.GetForSkillLevel(skillLevel, sdp.msgSeed)
		toAttackerRoomMsg = msgs.Separate.ToAttackerRoom.GetForSkillLevel(skillLevel, sdp.msgSeed)
		toDefenderRoomMsg = msgs.Separate.ToDefenderRoom.GetForSkillLevel(skillLevel, sdp.msgSeed)

		// Find the exit that leads to the target from the source (if any)
		if atkRoom := rooms.LoadRoom(sourceChar.RoomId); atkRoom != nil {
			for exitName, exit := range atkRoom.Exits {
				if exit.RoomId == targetChar.RoomId {
					tokenReplacements[items.TokenExitName] = exitName
					break
				}
			}
		}
		// find the exit that leads to the source from the target (if any)
		if defRoom := rooms.LoadRoom(targetChar.RoomId); defRoom != nil {
			for exitName, exit := range defRoom.Exits {
				if exit.RoomId == sourceChar.RoomId {
					tokenReplacements[items.TokenEntranceName] = exitName
					break
				}
			}
		}
	}

	if srcType == Mob {
		tokenReplacements[items.TokenSource] = sourceChar.GetMobName(0).String()
	}

	if tgtType == Mob {
		tokenReplacements[items.TokenTarget] = targetChar.GetMobName(0).String()
	}

	for tokenName, tokenValue := range tokenReplacements {
		toAttackerMsg = toAttackerMsg.SetTokenValue(tokenName, tokenValue)
		toDefenderMsg = toDefenderMsg.SetTokenValue(tokenName, tokenValue)
		toAttackerRoomMsg = toAttackerRoomMsg.SetTokenValue(tokenName, tokenValue)
		if len(string(toDefenderRoomMsg)) > 0 {
			toDefenderRoomMsg = toDefenderRoomMsg.SetTokenValue(tokenName, tokenValue)
		}
	}

	// Feint: replace miss messages with feint-flavored text for skilled attackers
	if isFeint {
		feintMsg := getFeintMessage()
		toAttackerMsg = items.ItemMessage(feintMsg.toAttacker)
		toDefenderMsg = items.ItemMessage(feintMsg.toDefender)
		toAttackerRoomMsg = items.ItemMessage(feintMsg.toRoom)
		// Apply name tokens to feint messages
		toAttackerMsg = toAttackerMsg.SetTokenValue(items.TokenTarget, tokenReplacements[items.TokenTarget])
		toAttackerMsg = toAttackerMsg.SetTokenValue(items.TokenTargetType, tokenReplacements[items.TokenTargetType])
		toDefenderMsg = toDefenderMsg.SetTokenValue(items.TokenSource, tokenReplacements[items.TokenSource])
		toDefenderMsg = toDefenderMsg.SetTokenValue(items.TokenSourceType, tokenReplacements[items.TokenSourceType])
		toAttackerRoomMsg = toAttackerRoomMsg.SetTokenValue(items.TokenSource, tokenReplacements[items.TokenSource])
		toAttackerRoomMsg = toAttackerRoomMsg.SetTokenValue(items.TokenSourceType, tokenReplacements[items.TokenSourceType])
		toAttackerRoomMsg = toAttackerRoomMsg.SetTokenValue(items.TokenTarget, tokenReplacements[items.TokenTarget])
		toAttackerRoomMsg = toAttackerRoomMsg.SetTokenValue(items.TokenTargetType, tokenReplacements[items.TokenTargetType])
	}

	if result.Crit {
		toAttackerMsg = items.ItemMessage(`<ansi fg="crit-text">***</ansi> ` + string(toAttackerMsg) + ` <ansi fg="crit-text">***</ansi>`)
		toDefenderMsg = items.ItemMessage(`<ansi fg="crit-text">***</ansi> ` + string(toDefenderMsg) + ` <ansi fg="crit-text">***</ansi>`)
		toAttackerRoomMsg = items.ItemMessage(`<ansi fg="crit-text">***</ansi> ` + string(toAttackerRoomMsg) + ` <ansi fg="crit-text">***</ansi>`)
		if len(string(toDefenderRoomMsg)) > 0 {
			toDefenderRoomMsg = items.ItemMessage(`<ansi fg="crit-text">***</ansi> ` + string(toDefenderRoomMsg) + ` <ansi fg="crit-text">***</ansi>`)
		}
	}

	if result.Fumble {
		toAttackerMsg = items.ItemMessage(`<ansi fg="fumble-text">!!!</ansi> ` + string(toAttackerMsg) + ` <ansi fg="fumble-text">!!!</ansi>`)
		toDefenderMsg = items.ItemMessage(`<ansi fg="fumble-text">!!!</ansi> ` + string(toDefenderMsg) + ` <ansi fg="fumble-text">!!!</ansi>`)
		toAttackerRoomMsg = items.ItemMessage(`<ansi fg="fumble-text">!!!</ansi> ` + string(toAttackerRoomMsg) + ` <ansi fg="fumble-text">!!!</ansi>`)
		if len(string(toDefenderRoomMsg)) > 0 {
			toDefenderRoomMsg = items.ItemMessage(`<ansi fg="fumble-text">!!!</ansi> ` + string(toDefenderRoomMsg) + ` <ansi fg="fumble-text">!!!</ansi>`)
		}
	}

	if len(prefix) > 0 {
		toAttackerMsg = items.ItemMessage(prefix + string(toAttackerMsg))
		toDefenderMsg = items.ItemMessage(prefix + string(toDefenderMsg))
		toAttackerRoomMsg = items.ItemMessage(prefix + string(toAttackerRoomMsg))
		if len(string(toDefenderRoomMsg)) > 0 {
			toDefenderRoomMsg = items.ItemMessage(prefix + string(toDefenderRoomMsg))
		}
	}

	// Per-swing hit-band Category from the weapon subtype.
	hitCat := CategoryForWeaponSubtype(ws.weaponSubType)

	// Send to attacker
	attackerMsg := string(toAttackerMsg)
	if attackSourceDamage > 0 && attackSourceReduction > 0 {
		attackerMsg += fmt.Sprintf(` <ansi fg="white">[%s was blocked]</ansi>`, GetDamageDescription(attackSourceReduction, sourceChar.HealthMax.Value))
	}

	result.SendToSource(hitCat, string(attackerMsg))

	// Send to victim
	defenderMsg := string(toDefenderMsg)
	if attackTargetDamage > 0 && attackTargetReduction > 0 {
		defenderMsg += fmt.Sprintf(` <ansi fg="red">[you blocked %s]</ansi>`, GetDamageDescription(attackTargetReduction, targetChar.HealthMax.Value))
	}

	result.SendToTarget(hitCat, string(defenderMsg))

	// Send to room
	result.SendToSourceRoom(hitCat,
		string(toAttackerRoomMsg.SetTokenValue(items.TokenTarget, targetChar.Name).
			SetTokenValue(items.TokenTargetType, string(tgtType))),
	)

	// Send to defender room if separate
	if len(string(toDefenderRoomMsg)) > 0 {
		result.SendToTargetRoom(hitCat,
			string(toDefenderRoomMsg.SetTokenValue(items.TokenTarget, targetChar.Name).SetTokenValue(items.TokenTargetType, string(tgtType))),
		)
	}
}

// applyPetDamage handles pet contribution to combat if applicable.
func applyPetDamage(result *AttackResult, sourceChar *characters.Character, targetChar *characters.Character, tgtType SourceTarget) {
	if petJoins, _ := dice.Percentile(20); !petJoins {
		return
	}
	if sourceChar.RoomId != targetChar.RoomId {
		return
	}
	if !sourceChar.Pet.Exists() || (sourceChar.Pet.Damage.BaseDamage <= 0 && sourceChar.Pet.Damage.DiceRoll == ``) {
		return
	}

	petDmg := sourceChar.Pet.Damage
	var petAttacks int
	var petBaseDmg, petVar float64
	if petDmg.BaseDamage > 0 {
		petAttacks = petDmg.Attacks
		if petAttacks < 1 {
			petAttacks = 1
		}
		petBaseDmg = float64(petDmg.BaseDamage)
		petVar = float64(petDmg.Variance)
	} else {
		petAttacks, _, _, _, _ = sourceChar.Pet.GetDiceRoll()
		petBaseDmg, petVar = dice.DiceToDistribution(petDmg.DiceCount, petDmg.SideCount, petDmg.BonusDamage)
	}

	// Pet bites and claws are physical damage, so they run through the target's
	// physical mitigation exactly like melee (buildDamageParams), ranged
	// (ExecuteSkillMove) and the mob-parity helpers. Before this, pet damage was
	// the one physical source that ignored armor entirely — an armored boss took
	// the same pet damage as an unarmored newbie.
	//
	// Both the mean AND the spread are mitigated. The pet's variance comes from
	// its authored dice spec, not from a stat, so it is not stat-proportional
	// (dice.Roll, not dice.RollStat, is correct here) — but scaling only the
	// mean would leave the roll's spread at full width against armor. Scaling
	// both by the same factor is exactly equivalent to mitigating the rolled
	// value, and preserves the pet's authored coefficient of variation.
	//
	// Pets have no crit path: nothing here inspects a Z-score or sets
	// result.Crit, so the "crits bypass mitigation" convention used by the melee
	// (combat_helpers.go), spell (combat_shared_helpers.go) and taunt
	// (combat_taunt.go) channels has nothing to attach to. Deliberately not
	// adding one — a pet crit is a separate design decision, not a bug fix.
	petMitigation := targetChar.GetPhysicalMitigation()
	petMitigationCap := MitigationCap(ChannelPhysical)
	petBaseDmg = ApplyMitigation(petBaseDmg, petMitigation, petMitigationCap)
	petVar = ApplyMitigation(petVar, petMitigation, petMitigationCap)

	// Pet damage is claws/bite/etc — natural-sharp band.
	petCat := messaging.CategoryHitNaturalSharp
	for i := 0; i < petAttacks; i++ {
		attackTargetDamage := int(math.Round(math.Max(0, dice.Roll(petBaseDmg, petVar).Value)))

		result.DamageToTarget += attackTargetDamage

		toAttackerMsg := fmt.Sprintf(`%s jumps into the fray and deals <ansi fg="damage">%s</ansi> to <ansi fg="%sname">%s</ansi>!`, sourceChar.Pet.DisplayName(), GetDamageDescription(attackTargetDamage, targetChar.HealthMax.Value), string(tgtType), targetChar.Name)
		result.SendToSource(petCat, toAttackerMsg)

		toDefenderMsg := fmt.Sprintf(`%s jumps into the fray and deals <ansi fg="damage">%s</ansi> to you!`, sourceChar.Pet.DisplayName(), GetDamageDescription(attackTargetDamage, targetChar.HealthMax.Value))
		result.SendToTarget(petCat, toDefenderMsg)

		toAttackerRoomMsg := fmt.Sprintf(`%s jumps into the fray and deals <ansi fg="damage">%s</ansi> to <ansi fg="%sname">%s</ansi>!`, sourceChar.Pet.DisplayName(), GetDamageDescription(attackTargetDamage, targetChar.HealthMax.Value), string(tgtType), targetChar.Name)
		result.SendToSourceRoom(petCat, toAttackerRoomMsg)
		if sourceChar.RoomId != targetChar.RoomId {
			result.SendToTargetRoom(petCat, toAttackerRoomMsg)
		}
	}
}

// feintMessage holds the three message variants for a feint.
type feintMessage struct {
	toAttacker string
	toDefender string
	toRoom     string
}

// feintMessages are weapon-agnostic feint flavor messages.
// Tokens: {target}/{targettype} for attacker POV, {source}/{sourcetype} for defender POV.
var feintMessages = []feintMessage{
	{
		toAttacker: `You feint at <ansi fg="{targettype}">{target}</ansi>, testing their defenses.`,
		toDefender: `<ansi fg="{sourcetype}">{source}</ansi> feints at you, probing for weakness.`,
		toRoom:     `<ansi fg="{sourcetype}">{source}</ansi> feints toward <ansi fg="{targettype}">{target}</ansi>, testing for openings.`,
	},
	{
		toAttacker: `You make a deliberate feint, drawing <ansi fg="{targettype}">{target}</ansi>'s guard wide.`,
		toDefender: `<ansi fg="{sourcetype}">{source}</ansi> feints deliberately, drawing your guard.`,
		toRoom:     `<ansi fg="{sourcetype}">{source}</ansi> makes a deliberate feint at <ansi fg="{targettype}">{target}</ansi>.`,
	},
	{
		toAttacker: `You throw a calculated misdirection at <ansi fg="{targettype}">{target}</ansi>.`,
		toDefender: `<ansi fg="{sourcetype}">{source}</ansi> throws a calculated misdirection your way.`,
		toRoom:     `<ansi fg="{sourcetype}">{source}</ansi> throws a misdirection at <ansi fg="{targettype}">{target}</ansi>.`,
	},
	{
		toAttacker: `You probe <ansi fg="{targettype}">{target}</ansi>'s defenses with a quick false strike.`,
		toDefender: `<ansi fg="{sourcetype}">{source}</ansi> probes your defenses with a quick false strike.`,
		toRoom:     `<ansi fg="{sourcetype}">{source}</ansi> probes <ansi fg="{targettype}">{target}</ansi>'s defenses with a quick feint.`,
	},
	{
		toAttacker: `You shift your weight and feint low, reading <ansi fg="{targettype}">{target}</ansi>'s reaction.`,
		toDefender: `<ansi fg="{sourcetype}">{source}</ansi> feints low, reading your reaction intently.`,
		toRoom:     `<ansi fg="{sourcetype}">{source}</ansi> feints low toward <ansi fg="{targettype}">{target}</ansi>, studying their stance.`,
	},
	{
		toAttacker: `You commit to a false opening, watching how <ansi fg="{targettype}">{target}</ansi> responds.`,
		toDefender: `<ansi fg="{sourcetype}">{source}</ansi> opens up deliberately, watching your response.`,
		toRoom:     `<ansi fg="{sourcetype}">{source}</ansi> makes a calculated false opening toward <ansi fg="{targettype}">{target}</ansi>.`,
	},
	{
		toAttacker: `You disguise a measuring strike as a real attack toward <ansi fg="{targettype}">{target}</ansi>.`,
		toDefender: `<ansi fg="{sourcetype}">{source}</ansi> disguises a measuring strike as a real attack.`,
		toRoom:     `<ansi fg="{sourcetype}">{source}</ansi> throws a measured feint toward <ansi fg="{targettype}">{target}</ansi>.`,
	},
	{
		toAttacker: `You draw <ansi fg="{targettype}">{target}</ansi>'s attention high with a deceptive flourish.`,
		toDefender: `<ansi fg="{sourcetype}">{source}</ansi> draws your attention with a deceptive flourish.`,
		toRoom:     `<ansi fg="{sourcetype}">{source}</ansi> flourishes deceptively toward <ansi fg="{targettype}">{target}</ansi>.`,
	},
}

// checkFeint returns true if a miss should be presented as an intentional feint.
// Probability scales smoothly from near-zero at rank 1 to ~33% at soft cap, capped at 75%.
func checkFeint(skillRank int) bool {
	if skillRank <= 0 {
		return false
	}
	bal := configs.GetBalanceConfig()
	softCap := float64(bal.SkillSoftCap)
	if softCap <= 0 {
		softCap = 50
	}
	ratio := float64(skillRank) / softCap
	feintChance := math.Min(0.75, 0.33*math.Pow(ratio, 1.5))
	return util.Rand(1000) < int(feintChance*1000)
}

// getFeintMessage returns a random feint message set.
func getFeintMessage() feintMessage {
	return feintMessages[util.Rand(len(feintMessages))]
}
