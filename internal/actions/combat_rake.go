package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// RakeResult holds the outcome of a rake attempt for the caller to use when
// formatting messages, firing events, and updating UI.
type RakeResult struct {
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

	// Executed reports whether the rake was actually performed. False when any
	// early-exit condition fired (OnCooldown, NoTarget).
	Executed bool

	// OnCooldown is true when the special-move cooldown blocked the rake.
	OnCooldown bool

	// Crafting is true when the actor is occupied by another activity.
	Crafting bool

	// NoTarget is true when there is no aggro target or the target is gone.
	NoTarget bool

	// NotClawed is true when the actor's species lacks a clawed natural attack.
	// Reachable via a direct player command or a btree/combatcommands dispatch
	// to a non-clawed mob. Unreachable via the AI path (CanUseRake gates it).
	NotClawed bool

	// BleedDmg is the per-round amount of the bleed stack added on a hit.
	BleedDmg int
}

// ExecuteRake performs the core rake resolution shared between player and mob
// callers. It handles:
//   - Special-move cooldown (using SpecialMoveCooldown from balance config)
//   - Target resolution via ResolveAggroTarget
//   - ExecuteSkillMove via combat package (UnarmedCombat skill, Dexterity
//     attack stat, Dexterity defense stat, TripDamagePercent, Strength damage
//     stat, no knockdown)
//   - On hit: add a Bleeding stack (RakeBleedRounds, RakeBleedStrengthDivisor,
//     RakeBleedMin) sourced as "rake"
//   - combat.RecordSpecialMove for analytics + RoundsWaiting = 1
//   - OnSkillUse(UnarmedCombat) on hit for progression
//
// Callers are responsible for all messaging and any combat-initiation logic.
func ExecuteRake(actor Actor) RakeResult {
	char := actor.GetCharacter()

	if char.IsActing() {
		return RakeResult{Crafting: true}
	}

	// Resolve the aggro target.
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return RakeResult{NoTarget: true}
	}

	// Anatomy/identity gate (defense-in-depth): only handless clawed creatures
	// rake. Unreachable via the AI path (CanUseRake gates it) but reachable
	// via a direct player command or a btree/combatcommands dispatch to a
	// non-clawed or tool-using mob.
	if char.HasBodyPart("hands") || !combat.SpeciesIsClawed(char) {
		return RakeResult{NotClawed: true}
	}

	cfg := configs.GetBalanceConfig()
	if !SpecialMoveReady(char) {
		return RakeResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionRake, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return RakeResult{Cost: cost}
	}
	if !ClaimSpecialMove(char) {
		return RakeResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

	// Execute the skill move (reuse trip's config for damage percent, no
	// knockdown — the clawed raking strike deals moderate damage and bleeds
	// rather than felling).
	// U6b Task 7: through the channel seam — raw rank in, the seam applies
	// SkillWeight (x1 -> x5 both sides); the defence is the equipment-gated
	// set, charged and progressed; the crit tier and fumble abort exist now.
	// Lighting plan 5b: the actor's room feeds both sight rows.
	sightRoom := combat.SightRoom(actor.GetRoom())
	result := combat.ExecuteSkillMove(combat.SkillMoveParams{
		Attacker: char,
		Defender: target.Char,
		Shape:    combatvocab.Melee(combatvocab.TargetSingle),
		Room:     sightRoom,
		Attack: combat.AttackSide{
			Stat: char.GetEffectiveDexterity(), StatName: "dexterity",
			Skill: skills.UnarmedCombat, SkillRank: char.GetSkillLevel(skills.UnarmedCombat),
			Mult:      combat.SituationalAttackMult(char, sightRoom, combatvocab.Melee(combatvocab.TargetSingle)),
			ForceCrit: combat.SleepingForceCrit(target.Char),
		},
		DamagePercent:   float64(cfg.TripDamagePercent),
		KnockdownFactor: 0, // No knockdown — bleed instead
		DamageStat:      char.Stats.Strength.ValueAdj,
	})

	// U6b Task 10: a crit-defended move earns the defender a counter-swing.
	counter := counterSkillMoveExit(actor, target.Char, result, combatvocab.Melee(combatvocab.TargetSingle), true)

	// On hit: add a bleed stack (RakeBleedRounds rounds, Strength /
	// RakeBleedStrengthDivisor per round, floor RakeBleedMin).
	bleedDmg := 0
	if result.Hit {
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.RakeBleedStrengthDivisor, cfg.RakeBleedMin)
		_ = target.Char.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, int(cfg.RakeBleedRounds), -float64(bleedDmg), "rake", ActorRefOf(actor))
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

	// Record combat analytics. result.Damage is always the amount actually
	// applied (hit, partial-on-defended, or 0 on a defensive crit), so it is
	// truthful whether or not the move landed.
	dmgRecorded := result.Damage
	combat.RecordSpecialMove(sourceType, targetType, "rake", result.Hit, dmgRecorded, char, target.Char, util.GetRoundCount())

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

	return RakeResult{
		Cost:       cost,
		Target:     target,
		MoveResult: result,
		Counter:    counter,
		Executed:   true,
		BleedDmg:   bleedDmg,
	}
}
