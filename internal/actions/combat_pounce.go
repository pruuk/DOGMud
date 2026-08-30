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

// PounceResult holds the outcome of a pounce attempt for the caller to use
// when formatting messages, firing events, and updating UI.
type PounceResult struct {
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

	// Executed reports whether the pounce was actually performed. False when
	// any early-exit condition fired (OnCooldown, NoTarget, Grappling,
	// NotPredator).
	Executed bool

	// OnCooldown is true when the special-move cooldown blocked the pounce.
	OnCooldown bool

	// Crafting is true when the actor is occupied by another activity.
	Crafting bool

	// NoTarget is true when there is no aggro target or the target is gone.
	NoTarget bool

	// Grappling is true when the actor is in any grapple state (Clinch through
	// Turtle). Pounce is a leaping move that requires freedom of movement —
	// can't be executed from a clinch or ground grapple. Reachable via a
	// direct player command or btree dispatch; unreachable via the AI path
	// (CanUsePounce gates it).
	Grappling bool

	// NotPredator is true when the actor's species is not a quadruped predator
	// (requires legs AND fanged-or-clawed). Reachable via a direct player
	// command or a btree/combatcommands dispatch to a non-predator mob.
	// Unreachable via the AI path (CanUsePounce gates it).
	NotPredator bool
}

// ExecutePounce performs the core pounce resolution shared between player and
// mob callers. It handles:
//   - Special-move cooldown (using SpecialMoveCooldown from balance config)
//   - Target resolution via ResolveAggroTarget
//   - Compound predator gate: legs AND (fanged OR clawed)
//   - ExecuteSkillMove via combat package (UnarmedCombat skill, Dexterity
//     attack stat, Dexterity defense stat, BashDamagePercent, Strength damage
//     stat, BashKnockdownFactor, KnockdownToSupine=true — backward knockdown)
//   - combat.RecordSpecialMove for analytics + RoundsWaiting = 1
//   - OnSkillUse(UnarmedCombat) on hit for progression
//
// Callers are responsible for all messaging and any combat-initiation logic.
func ExecutePounce(actor Actor) PounceResult {
	char := actor.GetCharacter()

	if char.IsActing() {
		return PounceResult{Crafting: true}
	}

	// Resolve the aggro target.
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return PounceResult{NoTarget: true}
	}

	// Grappling gate: pounce is a leaping move — can't execute from a clinch
	// or any ground grapple. Unreachable via the AI path (CanUsePounce gates
	// it) but reachable via a direct player command or btree dispatch.
	if char.IsGrappling() {
		return PounceResult{Grappling: true}
	}

	// Compound predator gate (defense-in-depth): only legged fanged-or-clawed
	// creatures pounce. Unreachable via the AI path (CanUsePounce gates it) but
	// reachable via a direct player command or btree dispatch to a non-predator.
	if !combat.SpeciesIsQuadrupedPredator(char) {
		return PounceResult{NotPredator: true}
	}

	cfg := configs.GetBalanceConfig()
	if !char.CooldownReady("special-move") {
		return PounceResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionPounce, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return PounceResult{Cost: cost}
	}
	if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return PounceResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

	// Execute the skill move — uses bash's damage percent and knockdown chance;
	// KnockdownToSupine=true drives the target backward (face-up). No bleed:
	// pounce is a knockdown opener, not a DoT.
	// U6b Task 7: through the channel seam — raw rank in, the seam applies
	// SkillWeight (x1 -> x5 both sides); the defence is the equipment-gated
	// set, charged and progressed; the crit tier and fumble abort exist now.
	result := combat.ExecuteSkillMove(combat.SkillMoveParams{
		Attacker: char,
		Defender: target.Char,
		Channel:  combat.ChannelMelee,
		Attack: combat.AttackSide{
			Stat: char.GetEffectiveDexterity(), StatName: "dexterity",
			Skill: skills.UnarmedCombat, SkillRank: char.GetSkillLevel(skills.UnarmedCombat),
			Mult:      combat.SituationalAttackMult(char, combat.ChannelMelee),
			ForceCrit: combat.SleepingForceCrit(target.Char),
		},
		DamagePercent:     float64(cfg.BashDamagePercent),
		KnockdownFactor:   float64(cfg.BashKnockdownFactor),
		KnockdownToSupine: true, // predator leaps and drives target backward
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
	combat.RecordSpecialMove(sourceType, targetType, "pounce", result.Hit, dmgRecorded, char, target.Char, util.GetRoundCount())

	// Consume the combat round.
	if char.CombatPhase != nil {
		char.SetRoundsWaiting(1)
	}

	// Progression: unarmed-combat on hit.
	// U10b-1 Task 18b: win OR lose. This was gated on the hit, so a special
	// move that missed trained nothing -- the same defect a failed craft had
	// before Task 16. The gate is now the AWARD WEIGHT rather than a
	// precondition; a thrown move is a resolved contest either way.
	actor.AwardResolved(result.Hit, actor.GetCharacter().CandidateFor(string(skills.UnarmedCombat)))

	return PounceResult{
		Cost:       cost,
		Target:     target,
		MoveResult: result,
		Counter:    counter,
		Executed:   true,
	}
}
