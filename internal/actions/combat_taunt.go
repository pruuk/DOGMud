package actions

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// TauntResult holds the outcome of a taunt/howl conviction attack for the
// caller to use when formatting messages and firing events.
type TauntResult struct {
	Cost characters.CostCommitResult

	// Target is the resolved aggro target. Valid only when Executed is true.
	Target AggroTarget

	// Executed reports whether the taunt was actually performed. False when any
	// early-exit condition fired (OnCooldown, NoTarget).
	Executed bool

	// OnCooldown is true when the special-move cooldown blocked the taunt.
	OnCooldown bool

	// NoTarget is true when there is no aggro target or the target is gone.
	NoTarget bool

	// Hit reports whether the conviction attack landed.
	Hit bool

	// Crit reports whether the attack was a critical success (normalized
	// opposed-roll margin >= combat.ContestCritThreshold).
	Crit bool

	// Fumble reports whether the attack was a critical failure (ZScore <= -2.0).
	Fumble bool

	// Crafting is true when the actor is mid-craft and cannot taunt.
	Crafting bool

	// Damage is the conviction damage dealt to the target on a hit.
	Damage int

	// DmgDesc is the player-facing description of the conviction damage.
	DmgDesc string

	// SelfDamage is the self-conviction damage taken on a fumble.
	SelfDamage int

	// Defence is the canonical defy contest outcome. Callers render this once
	// for their own audience and visibility rules.
	Defence combat.ChannelDefenceResult

	// AggroPulled is true when the taunt forced the target to switch aggro
	// to the taunter (target was fighting someone else).
	AggroPulled bool
}

// runTauntContest is the primary social-attack contest seam. The default is
// always the canonical contest runner; same-package tests replace it briefly
// to exercise admission, damage, defence, and aggro effects deterministically.
var runTauntContest = combat.RunContest

func tauntTargetIsCurrent(snapshot, current AggroTarget, originalRoomID int, char *characters.Character) bool {
	return snapshot.Found && current.Found &&
		snapshot.Char == current.Char &&
		snapshot.UserId == current.UserId &&
		snapshot.MobInstanceId == current.MobInstanceId &&
		char.RoomId == originalRoomID &&
		current.Char.RoomId == originalRoomID
}

// ExecuteTaunt performs the shared conviction-attack resolution used by both
// the player "taunt" command and the mob "howl" command. It handles:
//   - Aggro / cooldown guards
//   - Target resolution via ResolveAggroTarget
//   - Skill-weighted conviction vs. willpower opposed roll
//   - Conviction depletion penalty via ResourceMultiplier
//   - Fumble self-damage, normal hit with mitigation, crit bypass
//   - OnSkillUse progression trigger (fires on all outcomes)
//   - RecordSpecialMove analytics + RoundsWaiting consumption
//
// Callers are responsible for all player-facing messages and any
// out-of-combat aggro setup (e.g. player targeting before entering combat).
func ExecuteTaunt(actor Actor) TauntResult {
	char := actor.GetCharacter()
	originalRoomID := char.RoomId

	// Don't interrupt any active activity (cast/craft/salvage) to taunt.
	if char.IsActing() {
		return TauntResult{Crafting: true}
	}

	// Resolve the target through the staged-target-aware seam. Player wrappers
	// can validate a named opener here without setting aggro or seeding
	// aggression before admission.
	target := resolveActionTarget(actor, char)
	if !tauntTargetIsCurrent(target, target, originalRoomID, char) {
		return TauntResult{NoTarget: true}
	}
	targetSnapshot := target

	cfg := configs.GetBalanceConfig()
	if !char.CooldownReady("special-move") {
		return TauntResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionTaunt, characters.PoolConviction,
		float64(cfg.RhetoricActionBaseConvictionCost))
	if cost.Status == characters.CostRefused {
		return TauntResult{Cost: cost}
	}

	// The target can leave after the quote commits. The already-paid cost stays
	// paid, but a stale target must not consume cooldown, commit engagement,
	// reveal the actor, or resolve a contest.
	target = resolveActionTarget(actor, char)
	if !tauntTargetIsCurrent(targetSnapshot, target, originalRoomID, char) {
		return TauntResult{Cost: cost, NoTarget: true}
	}
	if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return TauntResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

	// Taunting is noisy only once the paid attempt owns the cooldown and target.
	if char.IsHidden() {
		// Best effort: the only failure modes are an activity veto or an
		// already-Revealing state, both of which mean the actor is not quietly
		// hidden any more, which is all this call is for.
		_ = char.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger:  awareness.TriggerNoisyAction,
			Metadata: map[string]any{"command": "taunt"},
		})
	}

	// Conviction attack: Charisma + (rhetoric * SkillWeight) vs
	//                    Willpower   + (rhetoric * SkillWeight).
	skillWeight := float64(cfg.SkillWeight)
	attackerRhetoric := float64(char.GetSkillLevel(skills.Rhetoric)) * skillWeight
	attackScore := float64(char.Stats.Charisma.ValueAdj) + attackerRhetoric

	defenderRhetoric := float64(target.Char.GetSkillLevel(skills.Rhetoric)) * skillWeight
	defenseScore := float64(target.Char.Stats.Willpower.ValueAdj) + defenderRhetoric

	// Apply smooth conviction-depletion penalty to hit chance. EffectivePoolMax,
	// not the raw max (U7 Task 11): current Conviction is already reserve-clamped,
	// so a raw denominator taxes a companion holder twice.
	cpPenalty := float64(cfg.ConvictionPenaltyMax)
	convMult := combat.ResourceMultiplier(char.Conviction, char.EffectivePoolMax(characters.PoolConviction), cpPenalty)
	attackScore *= convMult

	// Opposed roll for hit/miss/fumble/crit classification.
	res := runTauntContest(attackScore, []contest.Entry{{Score: defenseScore}})

	// Determine source/target types for analytics.
	sourceType := combat.User
	if !actor.IsPlayer() {
		sourceType = combat.Mob
	}
	targetType := combat.User
	if target.MobInstanceId > 0 {
		targetType = combat.Mob
	}

	// Fumble: self-conviction damage and early return.
	if res.AttackRoll.ZScore <= -2.0 {
		selfDmg := char.Stats.Charisma.ValueAdj / 10
		if selfDmg < 1 {
			selfDmg = 1
		}
		char.ApplyHarm(characters.PoolConviction, selfDmg,
			state.ActorRef{UserId: char.GetUserId(), MobInstanceId: char.MobInstanceId})

		actor.OnSkillUse(string(skills.Rhetoric))

		combat.RecordSpecialMove(sourceType, targetType, "taunt", false, 0,
			char, target.Char, util.GetRoundCount())

		if char.Aggro != nil {
			char.Aggro.RoundsWaiting = 1
		}

		return TauntResult{
			Cost:       cost,
			Target:     target,
			Executed:   true,
			Fumble:     true,
			SelfDamage: selfDmg,
		}
	}

	if res.Success {
		// Calculate conviction damage via the unified pipeline.
		rawDmg := combat.CalcRawDamage(
			char.Stats.Charisma.ValueAdj,
			int(attackerRhetoric),
			0.5, // taunt base item multiplier
			combat.ChannelConviction,
		)

		// Apply smooth conviction-depletion damage penalty.
		rawDmg *= convMult

		// Crit derives from the normalized opposed-roll margin, so beating the
		// target decisively is what lands a telling taunt. Fumbles deliberately
		// stay on the self-relative z-score, matching the melee path (5.11d).
		//
		// SIGN: contest.Result.Margin is ATTACK-positive and this is the ATTACKER's
		// crit check, so it is passed unnegated. The defensive mirror
		// (combat.ResolveChannelDefence) negates. Note this reads Result.Margin and
		// NOT AttackRoll.Margin: the core rolls via dice.Roll, which never populates a
		// roll's Margin field, so the latter would silently be zero and no taunt
		// would ever crit.
		isCrit := combat.AttackContestCrit(res.Margin, res.AttackRoll)

		// Chunk 5.11g: a crit bypasses the target's conviction mitigation AND
		// scales by the taunter's rhetoric rank. Before this, a crit against an
		// unmitigated target dealt exactly a normal hit's damage.
		dmg := combat.CritOrMitigatedDamage(
			rawDmg,
			int(attackerRhetoric),
			isCrit,
			target.Char.GetConvictionMitigation(),
			combat.MitigationCap(combat.ChannelConviction),
		)

		// U6 Task 12: the target mounts defy, on the shared resolver. This used to
		// be TryStoicResolve, a SECOND independent contest run on top of the
		// primary taunt roll above, returning a flat configured multiplier.
		// ResolveChannelDefence charges and progresses the defence itself, so the
		// defenderUserId this call site used to thread through is now read off the
		// defender.
		defence := combat.ChannelDefenceResult{DamageMultiplier: 1}
		if !isCrit {
			defence = combat.ResolveChannelDefence(combat.ChannelSocial, char, target.Char)
			mult := defence.DamageMultiplier
			if mult < 1.0 {
				dmg = int(math.Round(float64(dmg) * mult))
				if dmg < 1 && mult > 0 {
					dmg = 1
				}
			}
		}

		// Apply conviction damage to target.
		target.Char.ApplyHarm(characters.PoolConviction, dmg,
			state.ActorRef{UserId: char.GetUserId(), MobInstanceId: char.MobInstanceId})

		// Build player-facing damage description.
		convMaxRef := target.Char.ConvictionMax.Value
		if convMaxRef <= 0 {
			convMaxRef = 1
		}
		dmgDesc := combat.GetConvictionDamageDescription(dmg, convMaxRef)

		// Crit received: conviction progression for target player.
		if isCrit && target.UserId > 0 {
			target.Char.OnCritReceived("conviction", target.UserId)
		}

		actor.OnSkillUse(string(skills.Rhetoric))

		combat.RecordSpecialMove(sourceType, targetType, "taunt", true, dmg,
			char, target.Char, util.GetRoundCount())

		// Tank taunt: force mob target to switch aggro to the taunter.
		// Works for both player and mob taunters; the archetype tree for
		// tank_taunter relies on this mob-side path to prevent infinite
		// taunt loops.
		agroPulled := false
		if target.MobInstanceId > 0 {
			targetMob := mobs.GetInstance(target.MobInstanceId)
			if targetMob != nil && targetMob.Character.Aggro != nil {
				attackerUserId := actor.GetUserId()
				attackerMobId := actor.GetMobInstanceId()
				// ForceTauntAggro pins the target onto the taunter for
				// TauntHoldRounds so the reactive per-round `attack` re-aggro
				// can't immediately flip the target back to whoever is hitting
				// it. Without the hold, a single taunt is overwritten the next
				// time an ally swings (see combat_state_compat taunt-hold gate).
				holdRounds := int(cfg.TauntHoldRounds)
				if attackerUserId > 0 {
					// Player taunter.
					if targetMob.Character.Aggro.UserId != attackerUserId {
						targetMob.Character.ForceTauntAggro(attackerUserId, 0, holdRounds)
						agroPulled = true
					}
				} else if attackerMobId > 0 {
					// Mob taunter.
					if targetMob.Character.Aggro.MobInstanceId != attackerMobId || targetMob.Character.Aggro.UserId != 0 {
						targetMob.Character.ForceTauntAggro(0, attackerMobId, holdRounds)
						agroPulled = true
					}
				}
			}
		}

		if char.Aggro != nil {
			char.Aggro.RoundsWaiting = 1
		}

		return TauntResult{
			Cost:        cost,
			Target:      target,
			Executed:    true,
			Hit:         true,
			Crit:        isCrit,
			Damage:      dmg,
			DmgDesc:     dmgDesc,
			Defence:     defence,
			AggroPulled: agroPulled,
		}
	}

	// Miss.
	actor.OnSkillUse(string(skills.Rhetoric))

	combat.RecordSpecialMove(sourceType, targetType, "taunt", false, 0,
		char, target.Char, util.GetRoundCount())

	if char.Aggro != nil {
		char.Aggro.RoundsWaiting = 1
	}

	return TauntResult{
		Cost:     cost,
		Target:   target,
		Executed: true,
		Hit:      false,
	}
}
