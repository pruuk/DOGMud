package actions

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
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

	// Hit reports whether the taunt was delivered and contested (everything
	// except a fumble, since the U6b Task 5 collapse). There is no separate
	// miss outcome any more: a defensive win is a blunted, partial-damage
	// taunt, reported through Defence.Defended, exactly like a defended
	// spell cast.
	Hit bool

	// Crit reports the seam's margin-derived attacker crit verdict (the
	// rhetoric-vs-rhetoric pair bar via combat.CritBarFor, floor-guarded).
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

	// Counter reports the defy carve-out (U6b Task 10): a defy CRIT answers
	// with a free counter-taunt from the target, wired at this call site
	// because internal/combat can never call taunt resolution.
	Counter CounterTauntResult
}

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
//   - ONE channel contest via combat.ResolveChannelAttack (charisma +
//     rhetoric vs the target defy set), conviction depletion on the side Mult
//   - Fumble self-damage, full damage on an attack win, partial damage when
//     defy blunts it, crit bypass on the seam-derived crit verdict
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

	// ONE contest (U6b Task 5): Charisma + rhetoric x SkillWeight, conviction-
	// depleted, against the target's defy (Willpower + rhetoric x SkillWeight).
	// This used to be a hit gate here plus a SECOND defence contest on non-crit
	// hits, so the same defender score was contested twice; the seam is now the
	// contest, and it charges/progresses the defence and pays the crit/fumble
	// bonus tier itself.
	//
	// Apply the smooth conviction-depletion penalty via AttackSide.Mult — the
	// multiplier the old gate applied and the old defy leg omitted (spec 4.1);
	// the surviving score keeps it. EffectivePoolMax, not the raw max (U7 Task
	// 11): current Conviction is already reserve-clamped, so a raw denominator
	// taxes a companion holder twice.
	cpPenalty := float64(cfg.ConvictionPenaltyMax)
	convMult := combat.ResourceMultiplier(char.Conviction, char.EffectivePoolMax(characters.PoolConviction), cpPenalty)

	side := combat.AttackSide{
		Stat:      char.Stats.Charisma.ValueAdj,
		StatName:  "charisma",
		Skill:     skills.Rhetoric,
		SkillRank: char.GetSkillLevel(skills.Rhetoric),
		Mult:      convMult,
	}
	out := combat.ResolveChannelAttack(combat.ChannelSocial, side, char, target.Char)

	// Determine source/target types for analytics.
	sourceType := combat.User
	if !actor.IsPlayer() {
		sourceType = combat.Mob
	}
	targetType := combat.User
	if target.MobInstanceId > 0 {
		targetType = combat.Mob
	}

	// Fumble: self-conviction damage and early return. The seam's verdict is
	// self-relative and resolved BEFORE success (Assumption 7): a fumbled
	// taunt aborts even a winning roll, which is the pre-U6b behaviour made
	// explicit.
	if out.AttackerFumble {
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
			Defence:    out,
		}
	}

	// Calculate conviction damage via the unified pipeline. The rank input to
	// BOTH consumers is the RAW rhetoric rank (side.SkillRank, Assumption 8):
	// SkillMultiplier inside CalcRawDamage and CritDamageMultiplier inside
	// CritOrMitigatedDamage. The old code passed the x SkillWeight-weighted
	// score term to both, a x15.75-vs-x4.6 crit-multiplier outlier.
	rawDmg := combat.CalcRawDamage(
		char.Stats.Charisma.ValueAdj,
		side.SkillRank,
		0.5, // taunt base item multiplier
		combat.ChannelConviction,
	)

	// Apply smooth conviction-depletion damage penalty.
	rawDmg *= convMult

	// The crit verdict comes FROM the one contest: margin-derived against the
	// rhetoric-vs-rhetoric pair bar, floor-guarded, decided inside the seam.
	// Fumbles stayed self-relative and returned above.
	isCrit := out.AttackerCrit

	// Chunk 5.11g: a crit bypasses the target's conviction mitigation AND
	// scales by the taunter's rhetoric rank. Before this, a crit against an
	// unmitigated target dealt exactly a normal hit's damage.
	dmg := combat.CritOrMitigatedDamage(
		rawDmg,
		side.SkillRank,
		isCrit,
		target.Char.GetConvictionMitigation(),
		combat.MitigationCap(combat.ChannelConviction),
	)

	// The defence multiplier from the SAME contest: 1.0 on an attack win,
	// 0.0-0.5 when defy blunted it, 0.0 on a defensive crit. A defended taunt
	// deals partial damage, exactly like a defended spell cast — the old
	// separate miss outcome is gone with the gate.
	if mult := out.DamageMultiplier; mult < 1.0 {
		dmg = int(math.Round(float64(dmg) * mult))
		if dmg < 1 && mult > 0 {
			dmg = 1
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

	// Crit-received toughening now fires exactly once, inside the seam's
	// bonus tier (combat.ResolveChannelAttack -> awardChannelDefenceBonus);
	// the direct block this call site carried was a duplicate of it, deleted
	// with the collapse like Task 4's spell-side twins.

	actor.OnSkillUse(string(skills.Rhetoric))

	combat.RecordSpecialMove(sourceType, targetType, "taunt", !out.Defended, dmg,
		char, target.Char, util.GetRoundCount())

	// Tank taunt: force mob target to switch aggro to the taunter.
	// Works for both player and mob taunters; the archetype tree for
	// tank_taunter relies on this mob-side path to prevent infinite
	// taunt loops. A defended taunt still pulls: the words landed even if
	// the sting was defied (defensive-crit pulls are pre-collapse behaviour,
	// pinned by the channel-defence runtime test).
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

	// U6b Task 10, the defy carve-out: a defy CRIT counter-taunts, replacing
	// the counter-swing every other channel gets. Wired here — after the
	// taunt has fully resolved (aggro pull included) — because internal/combat
	// cannot call taunt resolution. The entry point bypasses cooldown, U8
	// admission cost, and aggro mutation, and can never earn a counter.
	counter := counterTauntExit(actor, char, target, out)

	return TauntResult{
		Cost:        cost,
		Target:      target,
		Executed:    true,
		Hit:         true,
		Crit:        isCrit,
		Damage:      dmg,
		DmgDesc:     dmgDesc,
		Defence:     out,
		AggroPulled: agroPulled,
		Counter:     counter,
	}
}
