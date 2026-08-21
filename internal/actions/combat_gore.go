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

// GoreResult holds the outcome of a gore attempt for the caller to use
// when formatting messages, firing events, and updating UI.
type GoreResult struct {
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

	// Executed reports whether the gore was actually performed. False when
	// any early-exit condition fired (OnCooldown, NoTarget, NotHorned).
	Executed bool

	// OnCooldown is true when the special-move cooldown blocked the gore.
	OnCooldown bool

	// Crafting is true when the actor is occupied by another activity.
	Crafting bool

	// NoTarget is true when there is no aggro target or the target is gone.
	NoTarget bool

	// NotHorned is true when the actor's species is not horned (requires
	// a gore natural attack). Reachable via a direct player command or a
	// btree/combatcommands dispatch to a non-horned mob. Unreachable via
	// the AI path (CanUseGore gates it).
	NotHorned bool
}

// ExecuteGore performs the core gore resolution shared between player and
// mob callers. It handles:
//   - Special-move cooldown (using SpecialMoveCooldown from balance config)
//   - Target resolution via ResolveAggroTarget
//   - Horned-species gate: SpeciesIsHorned (gore natural attack)
//   - ExecuteSkillMove via combat package (UnarmedCombat skill, Strength
//     attack stat, Dexterity defense stat, KickDamagePercent,
//     BashKnockdownFactor, KnockdownToSupine=true — forward charge knockback)
//   - combat.RecordSpecialMove for analytics + RoundsWaiting = 1
//   - OnSkillUse(UnarmedCombat) on hit for progression
//
// Callers are responsible for all messaging and any combat-initiation logic.
func ExecuteGore(actor Actor) GoreResult {
	char := actor.GetCharacter()

	if char.IsActing() {
		return GoreResult{Crafting: true}
	}

	// Resolve the aggro target.
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return GoreResult{NoTarget: true}
	}

	// Horned gate (defense-in-depth): only handless horned creatures gore.
	// Unreachable via the AI path (CanUseGore gates it) but reachable via a
	// direct player command or btree dispatch to a non-horned or tool-using mob.
	if char.HasBodyPart("hands") || !combat.SpeciesIsHorned(char) {
		return GoreResult{NotHorned: true}
	}

	cfg := configs.GetBalanceConfig()
	if !char.CooldownReady("special-move") {
		return GoreResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionGore, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return GoreResult{Cost: cost}
	}
	if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return GoreResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

	// Execute the skill move — uses kick's damage percent for the charge
	// impact and bash's knockdown chance; KnockdownToSupine=true drives the
	// target forward (face-up). No bleed: gore is a knockdown opener.
	// U6b Task 7: through the channel seam — raw rank in, the seam applies
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
		DamagePercent:     float64(cfg.KickDamagePercent),
		KnockdownFactor:   float64(cfg.BashKnockdownFactor),
		KnockdownToSupine: true, // horned charge drives target forward (face-up)
		DamageStat:        char.Stats.Strength.ValueAdj,
	})

	// U6b Task 10: a crit-defended move earns the defender a counter-swing.
	counter := counterSkillMoveExit(actor, target.Char, result, combat.ChannelMelee, true)

	// Determine source/target types for analytics.
	sourceType := combat.User
	if !actor.IsPlayer() {
		sourceType = combat.Mob
	}
	targetType := combat.User
	if target.MobInstanceId > 0 {
		targetType = combat.Mob
	}

	// Record combat analytics. result.Damage is always the amount actually
	// applied (hit, partial-on-defended, or 0 on a defensive crit), so it is
	// truthful whether or not the move landed.
	dmgRecorded := result.Damage
	combat.RecordSpecialMove(sourceType, targetType, "gore", result.Hit, dmgRecorded, char, target.Char, util.GetRoundCount())

	// Consume the combat round.
	if char.Aggro != nil {
		char.Aggro.RoundsWaiting = 1
	}

	// Progression: unarmed-combat on hit.
	if result.Hit {
		actor.OnSkillUse(string(skills.UnarmedCombat))
	}

	return GoreResult{
		Cost:       cost,
		Target:     target,
		MoveResult: result,
		Counter:    counter,
		Executed:   true,
	}
}
