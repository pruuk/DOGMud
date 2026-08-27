package hooks

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/control"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// =============================================================================
// Stage 38.1: Shared combat helpers for mob/player unification
//
// Each helper operates on *characters.Character so it works for both players
// and mobs. Callers handle messaging (since mob vs player messaging differs).
// =============================================================================

// calcSpellDamageForCharacter computes spell damage using the pipeline (if
// spell has DamageMultiplier > 0) or the legacy path. Works for any caster.
func calcSpellDamageForCharacter(spellData *spells.SpellData, caster *characters.Character, target *characters.Character, magnitude int, isCrit bool) int {

	if spellData.DamageMultiplier > 0 && caster != nil {
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
		skillLevel := caster.GetSkillLevel(castSkill)
		rawDmg := combat.CalcRawDamage(spellData.CasterStatValue(caster.Stats), skillLevel, spellData.DamageMultiplier, combat.ChannelMagical)

		// Mutation graph: spell-power passives (Ether Gland, Corvid Brain) amplify caster output.
		if spBonus := mutations.GetSpellPowerMultiplier(caster.Mutations); spBonus != 0 {
			rawDmg *= 1.0 + spBonus
		}

		// #22 crash-site: inside the buried hull, belief-driven power is suppressed.
		// (caster is already non-nil — the enclosing branch requires it.)
		if caster.HasBuffFlag(buffs.Dampened) {
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
		rawDmg *= combat.ResourceMultiplier(caster.Conviction,
			caster.EffectivePoolMax(characters.PoolConviction), cpPenalty)

		// Apply mitigation based on defense type
		var mitigPct, cap float64
		switch spellData.TargetDefenseType {
		case "physical":
			mitigPct = target.GetPhysicalMitigation()
			cap = combat.MitigationCap(combat.ChannelPhysical)
		case "mental":
			mitigPct = target.GetMagicalMitigation()
			cap = combat.MitigationCap(combat.ChannelMagical)
		default:
			mitigPct = 0
			cap = 0.75
		}

		// Chunk 5.11g: a crit bypasses mitigation AND scales by the caster's
		// spellcasting rank. Before this, a crit against an unmitigated target
		// dealt exactly a normal hit's damage, so crit worth was set purely by
		// the defender's armour.
		return combat.CritOrMitigatedDamage(rawDmg, skillLevel, isCrit, mitigPct, cap)
	}

	// Legacy fallback: magnitude-based damage (no mitigation, no stat scaling)
	mudlog.Warn("calcSpellDamageForCharacter", "spell", spellData.SpellId, "msg", "using legacy magnitude path — add damage_multiplier to spell YAML")
	dmgRoll := dice.RollStat(float64(magnitude))
	dmg := int(math.Round(dmgRoll.Value))
	if dmg < 1 {
		dmg = 1
	}
	if isCrit {
		dmg += magnitude
	}
	return dmg
}

// checkConcentrationBreak tests whether a casting character's concentration
// breaks from taking damage. Returns true if concentration broke.
// Caller is responsible for clearing Activity state and sending messages.
func checkConcentrationBreak(ch *characters.Character, damage int) bool {
	if ch.Activity == nil || !ch.Activity.IsCasting() || damage <= 0 {
		return false
	}
	// Telegraphed casts flagged NoDamageInterrupt only break to the
	// disruptor system's intended interrupts (thrown flashbang / disruption
	// spells) — incidental combat damage never breaks them.
	if cs, ok := ch.Activity.CastingData(); ok {
		if sd := spells.GetSpell(cs.SpellId); sd != nil && sd.NoDamageInterrupt {
			return false
		}
	}
	maxHealth := ch.HealthMax.Value
	damagePct := damage * 100 / maxHealth
	if damagePct < int(configs.GetBalanceConfig().ConcentrationDamageThresholdPct) {
		// Chip damage does not generate rolls at all (U10).
		return false
	}
	res := combat.RunConcentrationContest(concentrationScore(ch), float64(damagePct*10))
	// U10b-1 Task 12: win OR lose. This was success-only -- one spellcasting
	// event per HELD contest, nothing at all for a broken one -- which is the
	// convention this slice replaces. Holding against a blow and failing to
	// hold are both resolved contests; the second is exactly the case a caster
	// learns from.
	//
	// res.Success is the CASTER's win: RunConcentrationContest takes the
	// caster's score as the attack side, so Success means the hold beat the
	// disruption.
	//
	// Still gated by the chip-damage floor above, which returns before the
	// contest runs. Damage under ConcentrationDamageThresholdPct generates no
	// contest, so it awards nothing -- a caster does not learn from being
	// scratched.
	awardConcentration(ch, res.Success)
	return !res.Success
}

// awardConcentration fires the one progression event a resolved concentration
// contest earns, at full weight when the caster held and at
// Balance.ProgressionFailureFraction when the spell broke.
//
// One place for all three concentration triggers -- damage
// (checkConcentrationBreak), position (processFoldRound) and the throttle
// maneuver -- so the firing rule cannot drift between them. The throttle site
// lives in internal/actions and calls Character.AwardResolved directly for the
// same reason; only the two hooks-side callers share this helper.
//
// ⚠️ RATE. Concentration fires PER DAMAGE INSTANCE, so a caster taking four
// swings in a round resolves up to four contests, and every one of them now
// awards where previously only the held ones did. skills
// .SkillProgressionMultipliers fits spellcasting at 3.90 on the premise that
// casting is rare; that premise is now weaker on the concentration path than
// on the cast path. Carry it into the re-solve.
//
// The Candidate's Roll is unused: with a single candidate BestOf returns it
// whatever it rolled. CandidateFor is still the right constructor, because it
// is the one place that knows how to build a valid candidate and it refuses an
// unknown skill.
func awardConcentration(ch *characters.Character, held bool) {
	if ch == nil {
		return
	}
	ch.AwardResolved(ch.GetUserId(), held, ch.CandidateFor(string(skills.Spellcasting)))
}

// concentrationScore is the caster's side of every concentration contest:
// Wil + spellcasting x SkillWeight, the arc's standard shape.
func concentrationScore(ch *characters.Character) float64 {
	return float64(ch.Stats.Willpower.ValueAdj) +
		float64(ch.GetSkillLevel(skills.Spellcasting))*float64(configs.GetBalanceConfig().SkillWeight)
}

// WeaponBreakResult holds the outcome of a weapon break test.
type WeaponBreakResult struct {
	Broke           bool
	BrokenItemName  string
	BrokenItem      items.Item
	ReplacementItem items.Item
}

// tryWeaponBreak tests offhand breakage for any character and performs the
// item swap (remove broken offhand, create broken-item #20). Returns the
// result so callers can emit ItemOwnership events and messages.
// The room param is used as a fallback for item drops; nil-safe.
func tryWeaponBreak(defender *characters.Character, roundResult combat.AttackResult, room *rooms.Room) WeaponBreakResult {
	result := WeaponBreakResult{}

	// U6 Task 14: break-testing the defender's offhand keys on the attack
	// actually landing cleanly. A deflected swing (Hit true, defence won)
	// does not test breakage — preserving pre-U6 behavior, where a defended
	// swing was a miss. Crit implies CleanHit, so the crit modifier below is
	// unaffected.
	if !roundResult.CleanHit {
		return result
	}
	if defender.Equipment.Offhand.ItemId <= 0 {
		return result
	}

	modifier := 0
	if roundResult.Crit {
		modifier = int(defender.Equipment.Offhand.GetSpec().BreakChance)
	}

	if !defender.Equipment.Offhand.BreakTest(modifier) {
		return result
	}

	result.Broke = true
	result.BrokenItemName = defender.Equipment.Offhand.NameSimple()
	result.BrokenItem = defender.Equipment.Offhand

	defender.RemoveFromBody(defender.Equipment.Offhand)

	itm := items.New(20) // Broken item
	result.ReplacementItem = itm
	if !defender.StoreItem(itm) {
		if room != nil {
			room.AddItem(itm, false)
		}
	}

	return result
}

// CritEffectResult holds the outcome of applying defense crit effects.
type CritEffectResult struct {
	// Parry crit → riposte (free counter-attack)
	Riposte       bool
	RiposteDamage int
	RiposteMaxHP  int
	// Dodge crit → auto-trip (ignores cooldown)
	AutoTrip   bool
	TripResult combat.SkillMoveResult
	// Block crit → auto-bash (ignores cooldown)
	AutoBash   bool
	BashResult combat.SkillMoveResult
	// Messages for all crit effects
	DefenderMsg string
	AttackerMsg string
	RoomMsg     string
}

// charActorRef builds the ActorRef identifying a character as the SOURCE of
// harm, for characters.ApplyHarm.
//
// Nil-safe on purpose: several harm sites in this package hold a caster
// pointer that is legitimately nil (mob-cast paths with no originating
// character), and the pool primitives read the zero value as "anonymous",
// which is exactly the right answer there.
func charActorRef(c *characters.Character) state.ActorRef {
	if c == nil {
		return state.ActorRef{}
	}
	return state.ActorRef{UserId: c.GetUserId(), MobInstanceId: c.MobInstanceId}
}

// applyCritEffects processes parry/dodge/block crit effects for any combat
// pairing. The attacker is the one who swung. The defender is the one who
// rolled the defense crit and gains the benefit.
func applyCritEffects(attacker, defender *characters.Character, roundResult combat.AttackResult, room *rooms.Room) CritEffectResult {
	result := CritEffectResult{}
	cfg := configs.GetBalanceConfig()

	// ── Parry crit → riposte: free counter-swing ────────────────────────
	// U6b Task 10: the damage fraction is the CounterDamagePercent knob (the
	// same knob pricing the cross-channel counter tier; shipped 0.5, exactly
	// the old literal). 0 is the documented off-switch and must be handled
	// HERE: CalcRawDamage treats itemMult <= 0 as "unset" and substitutes
	// 0.30, which would turn the off-switch into a 30%-damage riposte. The
	// riposte otherwise keeps its historical uncontested maths — melee
	// behaviour is unchanged at the shipped value.
	counterPct := float64(cfg.CounterDamagePercent)
	if roundResult.ParryCritDetected && counterPct > 0 {
		raw := combat.CalcRawDamage(
			defender.Stats.Strength.ValueAdj,
			defender.GetCombatSkillLevel(),
			counterPct,
			combat.ChannelPhysical,
		)
		dmgMean := combat.ApplyMitigation(raw, attacker.GetPhysicalMitigation(),
			combat.MitigationCap(combat.ChannelPhysical))
		roll := dice.RollStat(dmgMean)
		dmg := int(roll.Value)
		if dmg < 1 {
			dmg = 1
		}

		// Reassigned from the APPLIED return so result.RiposteDamage and the
		// damage description below cannot drift from what the pool actually
		// took.
		dmg = attacker.ApplyHarm(characters.PoolHealth, dmg, charActorRef(defender))
		result.Riposte = true
		result.RiposteDamage = dmg
		result.RiposteMaxHP = attacker.HealthMax.Value

		dmgDesc := combat.GetDamageDescription(dmg, attacker.HealthMax.Value)
		result.DefenderMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ RIPOSTE!</ansi> You turn the parry into a swift counter-strike! (<ansi fg="damage">%s</ansi>)`, dmgDesc)
		result.AttackerMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ RIPOSTE!</ansi> %s turns the parry into a lightning counter-strike! (<ansi fg="damage">%s</ansi>)`, defender.Name, dmgDesc)
		result.RoomMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ RIPOSTE!</ansi> %s turns a deft parry into a counter-strike against %s!`, defender.Name, attacker.Name)
	}

	// ── Dodge crit → auto-trip (ignores cooldown) ───────────────────────
	if roundResult.DodgeCritDetected {
		// U6b Task 7: through the channel seam. IsCounter marks this as a
		// move executed AS a counter — consumed by Task 10 (a counter must
		// never trigger another counter); behaviour-neutral until then.
		tripResult := combat.ExecuteSkillMove(combat.SkillMoveParams{
			Attacker: defender,
			Defender: attacker,
			Channel:  combat.ChannelMelee,
			Attack: combat.AttackSide{
				Stat: defender.GetEffectiveDexterity(), StatName: "dexterity",
				Skill: skills.UnarmedCombat, SkillRank: defender.GetSkillLevel(skills.UnarmedCombat),
				// Task 17: the countering character pays their own prone /
				// stamina-depletion accuracy terms. No ForceCrit: the counter
				// target just swung, so they cannot be sleeping.
				Mult: combat.SituationalAttackMult(defender, combat.ChannelMelee),
			},
			IsCounter:       true,
			DamagePercent:   float64(cfg.TripDamagePercent),
			KnockdownFactor: float64(cfg.TripKnockdownFactor),
			DamageStat:      defender.GetEffectiveDexterity(),
		})
		result.AutoTrip = true
		result.TripResult = tripResult

		if tripResult.Hit {
			dmgDesc := combat.GetDamageDescription(tripResult.Damage, tripResult.TargetMaxHP)
			if tripResult.KnockedDown {
				result.DefenderMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> You dodge and sweep their legs out! They crash to the ground! (<ansi fg="damage">%s</ansi>)`, dmgDesc)
				result.AttackerMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> %s dodges and sweeps your legs! You crash to the ground! (<ansi fg="damage">%s</ansi>)`, defender.Name, dmgDesc)
				result.RoomMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> %s dodges and sweeps %s to the ground!`, defender.Name, attacker.Name)
			} else {
				result.DefenderMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> You dodge and lash out at their legs! (<ansi fg="damage">%s</ansi>)`, dmgDesc)
				result.AttackerMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> %s dodges and lashes out at your legs! (<ansi fg="damage">%s</ansi>)`, defender.Name, dmgDesc)
				result.RoomMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> %s dodges and kicks at %s's legs!`, defender.Name, attacker.Name)
			}
		} else if tripResult.Damage > 0 {
			dmgDesc := combat.GetDamageDescription(tripResult.Damage, tripResult.TargetMaxHP)
			result.DefenderMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> You dodge and lash at their legs. They stay up, but you still catch them! (<ansi fg="damage">%s</ansi>)`, dmgDesc)
			result.AttackerMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> %s dodges and lashes at your legs. You stay up, but it still catches you! (<ansi fg="damage">%s</ansi>)`, defender.Name, dmgDesc)
			result.RoomMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> %s dodges and lashes at %s, who stays up but takes the blow!`, defender.Name, attacker.Name)
		} else {
			result.DefenderMsg = `<ansi fg="cyan-bold">⚡ SWEEP!</ansi> You dodge and try to sweep their legs, but they keep their footing!`
			result.AttackerMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> %s dodges and tries to sweep your legs, but you keep your footing!`, defender.Name)
			result.RoomMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">⚡ SWEEP!</ansi> %s dodges and tries to sweep %s's legs, but misses!`, defender.Name, attacker.Name)
		}
	}

	// ── Block crit → auto-bash (ignores cooldown) ───────────────────────
	if roundResult.BlockCritDetected {
		// U6b Task 7: through the channel seam. IsCounter marks this as a
		// move executed AS a counter — consumed by Task 10 (a counter must
		// never trigger another counter); behaviour-neutral until then.
		bashResult := combat.ExecuteSkillMove(combat.SkillMoveParams{
			Attacker: defender,
			Defender: attacker,
			Channel:  combat.ChannelMelee,
			Attack: combat.AttackSide{
				Stat: defender.Stats.Strength.ValueAdj, StatName: "strength",
				Skill: skills.WeaponCombat, SkillRank: defender.GetSkillLevel(skills.WeaponCombat),
				// Task 17: the countering character pays their own prone /
				// stamina-depletion accuracy terms. No ForceCrit: the counter
				// target just swung, so they cannot be sleeping.
				Mult: combat.SituationalAttackMult(defender, combat.ChannelMelee),
			},
			IsCounter:         true,
			DamagePercent:     float64(cfg.BashDamagePercent),
			KnockdownFactor:   float64(cfg.BashKnockdownFactor),
			DamageStat:        defender.Stats.Strength.ValueAdj,
			KnockdownToSupine: true, // bash sends attacker backward
		})
		result.AutoBash = true
		result.BashResult = bashResult

		if bashResult.Hit {
			dmgDesc := combat.GetDamageDescription(bashResult.Damage, bashResult.TargetMaxHP)
			if bashResult.KnockedDown {
				result.DefenderMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> You catch the blow on your shield and slam them back! They stumble and fall! (<ansi fg="damage">%s</ansi>)`, dmgDesc)
				result.AttackerMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> %s catches your blow on their shield and slams you back! You stumble and fall! (<ansi fg="damage">%s</ansi>)`, defender.Name, dmgDesc)
				result.RoomMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> %s blocks and shield-slams %s to the ground!`, defender.Name, attacker.Name)
			} else {
				result.DefenderMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> You catch the blow and slam your shield into them! (<ansi fg="damage">%s</ansi>)`, dmgDesc)
				result.AttackerMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> %s catches your blow and slams their shield into you! (<ansi fg="damage">%s</ansi>)`, defender.Name, dmgDesc)
				result.RoomMsg = fmt.Sprintf(
					`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> %s blocks and slams their shield into %s!`, defender.Name, attacker.Name)
			}
		} else if bashResult.Damage > 0 {
			dmgDesc := combat.GetDamageDescription(bashResult.Damage, bashResult.TargetMaxHP)
			result.DefenderMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> You catch the blow and try to slam them. They brace against it, but your shield still connects! (<ansi fg="damage">%s</ansi>)`, dmgDesc)
			result.AttackerMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> %s catches your blow and tries to slam you. You brace against it, but the shield still connects! (<ansi fg="damage">%s</ansi>)`, defender.Name, dmgDesc)
			result.RoomMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> %s blocks and tries to slam %s, who braces but still takes the hit!`, defender.Name, attacker.Name)
		} else {
			result.DefenderMsg = `<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> You catch the blow and try to slam them, but they brace against it!`
			result.AttackerMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> %s catches your blow and tries to slam you, but you brace against it!`, defender.Name)
			result.RoomMsg = fmt.Sprintf(
				`<ansi fg="cyan-bold">🛡 SHIELD SLAM!</ansi> %s blocks and tries to slam %s, but they hold firm!`, defender.Name, attacker.Name)
		}
	}

	return result
}

// simulateFoldRound simulates the fold doubling loop to determine how many
// folds will be gained this round (without actually modifying state).
// Returns the foldDelta.
func simulateFoldRound(cs activity.CastingData) int {
	simFolds := cs.FoldsAccumulated
	for i := 0; i < cs.FoldsPerRound; i++ {
		if simFolds == 0 {
			simFolds = 1
		} else {
			simFolds *= 2
		}
		if simFolds >= cs.FoldsNeeded {
			simFolds = cs.FoldsNeeded
			break
		}
	}
	return simFolds - cs.FoldsAccumulated
}

// calcFoldConvictionCost computes the conviction cost for a fold round,
// proportional to folds gained vs total folds needed.
func calcFoldConvictionCost(cs activity.CastingData, foldDelta int) int {
	if cs.TotalConvictionCost <= 0 || cs.FoldsNeeded <= 0 {
		return 0
	}
	roundCost := (cs.TotalConvictionCost * foldDelta) / cs.FoldsNeeded
	if roundCost < 1 {
		roundCost = 1
	}
	return roundCost
}

// clearCastingActivity fires Activity.TransitionToFree with the given trigger
// if the character's Activity machine is currently in the Casting state.
func clearCastingActivity(ch *characters.Character, trigger string) {
	if ch.Activity != nil && ch.Activity.IsCasting() {
		_ = ch.Activity.TransitionToFree(state.TransitionReason{
			Trigger: trigger,
			Actor:   ch.Activity.Self(),
		})
	}
}

// cancelCraftOrSalvageOnDamage cancels the character's Activity if it is
// Crafting or Salvaging. This is a hard cancel — no roll — because any
// damage interrupts physical work immediately. Casting interrupts are
// handled separately by clearCastingActivity (concentration break via
// willpower roll). Both helpers may be called together at the same damage
// site: they are independent and each no-ops when the state doesn't match.
func cancelCraftOrSalvageOnDamage(ch *characters.Character) {
	if ch.Activity == nil {
		return
	}
	switch ch.Activity.State() {
	case activity.Crafting:
		_ = ch.Activity.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerDamageInterrupt,
			Actor:   ch.Activity.Self(),
		})
	case activity.Salvaging:
		_ = ch.Activity.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerDamageInterrupt,
			Actor:   ch.Activity.Self(),
		})
	}
}

// cancelDamageBuffs fires the Sleeping wake hook (if the character is asleep)
// and then cancels all active buffs with the CancelOnDamage flag.
//
// Call this immediately after any damage > 0 is committed to a character's
// Health — melee hits, spell hits, DoT ticks.
//
// Order matters: HasBuffFlag(Sleeping) is read BEFORE CancelBuffsWithFlag
// removes the buff; if we checked after the cancel, the flag would already
// be gone and OnSleeperWoken would never fire.
func cancelDamageBuffs(ch *characters.Character) {
	if ch.HasBuffFlag(buffs.Sleeping) {
		mobs.OnSleeperWoken(ch)
	}
	ch.CancelBuffsWithFlag(buffs.CancelOnDamage)
}

// =============================================================================
// processFoldRound — shared fold casting step for both players and mobs.
//
// Handles all state mutations that are identical for every caster type:
//   - Prone → concentration break
//   - Target-gone check
//   - Simulate fold advance → compute conviction cost
//   - Insufficient conviction → break
//   - Deduct conviction + advance folds
//
// The caller (handlePlayerFoldCasting / handleMobFoldCasting) is responsible
// for all messaging and spell resolution, because those differ by caster type.
// =============================================================================

// FoldRoundResult describes the outcome of a single fold casting step.
// Exactly one of the boolean fields will be true; StillCasting and
// CastComplete are mutually exclusive; the break fields are all terminal.
type FoldRoundResult struct {
	// Terminal states — caller should return after messaging.
	ProneBroke             bool // caster fell prone, concentration broken
	GrappleBroke           bool // caster is in a grapple state, concentration broken (chunk 4e T4)
	TargetGone             bool // all targets are dead/gone
	SpellDataMissing       bool // spells.GetSpell returned nil
	InsufficientConviction bool // not enough CP to pay this fold's cost

	// Ongoing states.
	StillCasting bool // folds advanced but not yet complete; send in-progress msg
	CastComplete bool // folds complete; caller should resolve the spell

	// Values the caller needs for messaging / resolution.
	FoldDelta      int                  // folds simulated this round
	ConvictionCost int                  // CP deducted from caster this round
	SpellData      *spells.SpellData    // non-nil when SpellDataMissing==false
	CastingData    activity.CastingData // snapshot of casting state at decision time
}

// processFoldRound advances one round of fold casting for any caster character.
// It uses the Activity machine as sole source of truth for CastingData.
// Returns a FoldRoundResult describing what happened so the caller can emit
// the right messages.
func processFoldRound(char *characters.Character) FoldRoundResult {
	if char.Activity == nil || !char.Activity.IsCasting() {
		return FoldRoundResult{}
	}
	cs, _ := char.Activity.CastingData()

	// Fetch the spell being cast up-front so both the position-disruption
	// check below and the damage-path checkConcentrationBreak (called by
	// the combat handlers) can consult NoDamageInterrupt. Telegraphed boss
	// casts (e.g. Core Guardian's core-discharge/core-drain) set this flag
	// so they only break to the disruptor system's intended interrupts,
	// never to incidental party damage or position churn.
	spellData := spells.GetSpell(cs.SpellId)

	// Position-based concentration disruption (chunk 4f, contested since
	// U10). Replaces the three deterministic 100% gates (Prone/Supine/
	// Grapple) that chunks pre-4e shipped. Now: damage%-equivalent per
	// (position, role) → x10 → combat.RunConcentrationContest against the
	// caster's concentrationScore, the arc's standard contest shape.
	// Standing returns 0 and skips the check entirely.
	//
	// The damage-path checkConcentrationBreak still fires independently
	// when damage lands during a round — both paths can break a single
	// cast (layered disruption) — unless the spell is flagged
	// NoDamageInterrupt, in which case both paths are skipped.
	if char.Position != nil && (spellData == nil || !spellData.NoDamageInterrupt) {
		posState := char.Position.State()
		var ctrlState control.State
		if char.Control != nil {
			ctrlState = char.Control.State()
		}
		dmgPctEquiv := position.PositionDisruptionDmgEquiv(posState, ctrlState)
		if dmgPctEquiv > 0 {
			// Chunk 4f's lattice keeps its full granularity; the x10
			// conversion is the design (owner 2026-08-21, re-ratified over
			// the corrected table — prone 300, deep holds 600-700).
			res := combat.RunConcentrationContest(concentrationScore(char), float64(dmgPctEquiv*10))
			// U10b-1 Task 12: awarded BEFORE the branch, so it fires win or
			// lose. It used to sit inside the success arm only. Note the loss
			// arm returns, so an award placed there would never run.
			awardConcentration(char, res.Success)
			if res.Success {
				// Held: nothing further. The award above already fired.
			} else {
				// Concentration broke. Route messaging by which break
				// flag the caller expects for this position. Default
				// falls back to GrappleBroke for any future non-Standing
				// position that lands here without a Prone/Supine/Grapple
				// classification.
				clearCastingActivity(char, activity.TriggerConcentrationBreak)
				result := FoldRoundResult{CastingData: cs}
				switch {
				case char.IsProne(), char.IsSupine():
					result.ProneBroke = true
				case char.IsGrappling():
					result.GrappleBroke = true
				default:
					result.GrappleBroke = true
				}
				return result
			}
			// Contest held — the fold continues this round.
		}
	}

	// Target-gone check: any dead/nil target breaks the spell.
	targetGone := false
	for _, mobInstId := range cs.TargetMobInstanceIds {
		m := mobs.GetInstance(mobInstId)
		if m == nil || m.Character.Health < 1 {
			targetGone = true
			break
		}
	}
	if !targetGone {
		for _, targetUserId := range cs.TargetUserIds {
			u := users.GetByUserId(targetUserId)
			if u == nil {
				targetGone = true
				break
			}
			// For harm spells, downed players count as gone.
			if u.Character.Health < 1 && spellData != nil &&
				(spellData.Type == spells.HarmSingle || spellData.Type == spells.HarmArea || spellData.Type == spells.HarmMulti) {
				targetGone = true
				break
			}
		}
	}
	if targetGone {
		clearCastingActivity(char, activity.TriggerConcentrationBreak)
		return FoldRoundResult{TargetGone: true, CastingData: cs}
	}

	if spellData == nil {
		clearCastingActivity(char, activity.TriggerConcentrationBreak)
		return FoldRoundResult{SpellDataMissing: true, CastingData: cs}
	}

	// Simulate fold advance → compute conviction cost.
	foldDelta := simulateFoldRound(cs)
	roundCost := calcFoldConvictionCost(cs, foldDelta)

	// Upkeep REFUSES when unaffordable: the cast collapses. ApplyCost pays in
	// full or takes nothing, so a broken concentration never also drains the
	// pool. The roundCost > 0 guard is now redundant -- ApplyCost treats a
	// non-positive amount as free and succeeds -- but it is kept so a zero-cost
	// fold cannot be read as a refusal path.
	//
	// Categorisation note: the refuse column's usual rationale ("the actor keeps
	// every other action") does NOT hold here. handlePlayerFoldCasting returns
	// true on every branch, so a caster who cannot pay loses the conviction
	// already sunk into prior folds, the spell, AND their round. That shape is
	// inherited, not introduced here; filed for U7/U8.
	if roundCost > 0 && !char.ApplyCost(characters.PoolConviction, roundCost) {
		clearCastingActivity(char, activity.TriggerConcentrationBreak)
		return FoldRoundResult{
			InsufficientConviction: true,
			FoldDelta:              foldDelta,
			ConvictionCost:         roundCost,
			SpellData:              spellData,
			CastingData:            cs,
		}
	}

	// Conviction was charged by the ApplyCost above. Advance folds via the
	// Activity machine.
	updatedCs, complete := char.Activity.AdvanceCastingFolds(foldDelta, roundCost)
	if complete {
		clearCastingActivity(char, activity.TriggerCastComplete)
		return FoldRoundResult{
			CastComplete:   true,
			FoldDelta:      foldDelta,
			ConvictionCost: roundCost,
			SpellData:      spellData,
			CastingData:    updatedCs,
		}
	}
	return FoldRoundResult{
		StillCasting:   true,
		FoldDelta:      foldDelta,
		ConvictionCost: roundCost,
		SpellData:      spellData,
		CastingData:    updatedCs,
	}
}
