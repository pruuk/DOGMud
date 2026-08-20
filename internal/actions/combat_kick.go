package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// KickVariant identifies which kick animation/variant was executed.
type KickVariant int

const (
	// KickStandard is a regular standing kick.
	KickStandard KickVariant = iota
	// KickStomp is used when the target is prone — lower-body stomp, extended
	// prone duration on hit, reduced mitigation for attacker.
	KickStomp
	// KickKnee is used when the attacker is in a grapple (clinched or grounded)
	// and holds the controller condition — a short-range knee strike.
	KickKnee
)

// KickResult holds the outcome of a kick attempt for the caller to use when
// formatting messages, firing events, and updating UI.
type KickResult struct {
	Cost characters.CostCommitResult

	// Target is the resolved aggro target. Valid only when Executed is true.
	Target AggroTarget

	// MoveResult is the outcome from ExecuteSkillMove. Valid only when Executed
	// is true.
	MoveResult combat.SkillMoveResult

	// Counter is the counter tier outcome (U6b Tasks 10-11): non-zero when the
	// defender crit-defended and answered. The command wrapper speaks its
	// narration AFTER the move's own outcome via DispatchCounterMessages.
	Counter combat.CounterResult

	// Variant reports which kick form was used.
	Variant KickVariant

	// Executed reports whether the kick was actually performed. False when any
	// early-exit condition fired (OnCooldown, NoTarget).
	Executed bool

	// OnCooldown is true when the special-move cooldown blocked the kick.
	OnCooldown bool

	// Crafting is true when the actor is mid-craft and cannot kick.
	Crafting bool

	// NoTarget is true when there is no aggro target or the target is gone.
	NoTarget bool
}

// ExecuteKick performs the core kick resolution shared between player and mob
// callers. It handles:
//   - Special-move cooldown (using SpecialMoveCooldown from balance config)
//   - Variant detection: stomp (target prone) or knee (grapple + controller)
//   - Target resolution via ResolveAggroTarget
//   - ExecuteSkillMove via combat package with variant-specific params
//   - Stomp extends prone duration by 1 round on a successful hit
//   - RecordAndWait (analytics + round consumption)
//
// Callers are responsible for all messaging, skill progression events, and
// any combat-initiation logic (e.g. SetAggro for player out-of-combat kick).
// raptorLegsKickBonus boosts standard-kick damage and knockdown when the
// attacker has the Raptor Legs mutation (digitigrade, talon-clawed legs).
// Returns the (possibly) adjusted damagePercent and knockdownChance.
func raptorLegsKickBonus(owned map[string]int, damagePercent float64, knockdownChance int) (float64, int) {
	if _, ok := owned["raptor-legs"]; ok {
		damagePercent += 0.20
		knockdownChance += 15
	}
	return damagePercent, knockdownChance
}

func ExecuteKick(actor Actor) KickResult {
	char := actor.GetCharacter()

	// Don't interrupt any active activity (cast/craft/salvage) to kick.
	if char.IsActing() {
		return KickResult{Crafting: true}
	}

	// Resolve the aggro target.
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return KickResult{NoTarget: true}
	}

	// Defense-in-depth anatomy gate; unreachable for players, AI/readiness gate upstream.
	if !char.HasBodyPart("legs") {
		return KickResult{NoTarget: true}
	}

	cfg := configs.GetBalanceConfig()
	if !char.CooldownReady("special-move") {
		return KickResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionKick, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return KickResult{Cost: cost}
	}
	if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return KickResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

	// Determine kick variant and associated params.
	variant := KickStandard
	damagePercent := float64(cfg.KickDamagePercent)
	knockdownChance := int(cfg.KickKnockdownChance)
	mitigationMult := 1.0

	// Stomp: target is downed (prone or supine, not grappled).
	if target.Char.IsProne() || target.Char.IsSupine() {
		variant = KickStomp
		damagePercent = float64(cfg.StompDamagePercent)
		knockdownChance = 0
		mitigationMult = 0.5
	}

	// Knee: attacker is in a grapple AND holds grapple control.
	// Knee overrides stomp (edge case: attacker grounded, target also prone).
	if char.IsGrappling() && char.IsController() {
		variant = KickKnee
		damagePercent = float64(cfg.KneeDamagePercent)
		knockdownChance = 0
		mitigationMult = 1.0
	}

	// Raptor Legs mutation: talon-clawed legs make a plain kick bite far harder.
	if variant == KickStandard {
		damagePercent, knockdownChance = raptorLegsKickBonus(char.Mutations, damagePercent, knockdownChance)
	}

	// Execute the skill move.
	// U6b Task 6: through the channel seam — raw rank in, the seam applies
	// SkillWeight (x1 -> x5 both sides); the defence is the equipment-gated
	// set, charged and progressed; the crit tier and fumble abort exist now.
	result := combat.ExecuteSkillMove(combat.SkillMoveParams{
		Attacker: char,
		Defender: target.Char,
		Channel:  combat.ChannelMelee,
		Attack: combat.AttackSide{
			Stat: char.Stats.Strength.ValueAdj, StatName: "strength",
			Skill: skills.UnarmedCombat, SkillRank: char.GetSkillLevel(skills.UnarmedCombat),
			Mult:      combat.SituationalAttackMult(char, combat.ChannelMelee),
			ForceCrit: combat.SleepingForceCrit(target.Char),
		},
		DamagePercent:        damagePercent,
		KnockdownChance:      knockdownChance,
		DamageStat:           char.Stats.Strength.ValueAdj,
		MitigationMultiplier: mitigationMult,
	})

	// U6b Task 10: a crit-defended move earns the defender a counter-swing.
	counter := counterSkillMoveExit(actor, target.Char, result, combat.ChannelMelee, true)

	// Stomp extends prone duration on a successful hit.
	if result.Hit && variant == KickStomp &&
		(target.Char.IsProne() || target.Char.IsSupine()) {
		target.Char.Position.ExtendRecoveryRound()
	}

	// Determine source/target types for analytics.
	sourceType := combat.User
	if !actor.IsPlayer() {
		sourceType = combat.Mob
	}
	targetType := combat.User
	if target.MobInstanceId > 0 {
		targetType = combat.Mob
	}

	// Choose the move name for analytics based on variant.
	moveName := "kick"
	switch variant {
	case KickStomp:
		moveName = "stomp"
	case KickKnee:
		moveName = "knee"
	}

	// Record analytics and consume the combat round. result.Damage is always
	// the amount actually applied (hit, partial-on-defended, or 0 on a
	// defensive crit), so it is truthful whether or not the move landed.
	dmgRecorded := result.Damage
	RecordAndWait(char, moveName, sourceType, target.Char, targetType, result.Hit, dmgRecorded, util.GetRoundCount())

	// Progression: unarmed-combat on hit (moved from user/mob wrappers)
	if result.Hit {
		actor.OnSkillUse(string(skills.UnarmedCombat))
	}

	return KickResult{
		Cost:       cost,
		Target:     target,
		MoveResult: result,
		Counter:    counter,
		Variant:    variant,
		Executed:   true,
	}
}
