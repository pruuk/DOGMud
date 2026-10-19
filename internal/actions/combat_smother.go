package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// SmotherResult holds the outcome of a smother attempt for the mob command.
type SmotherResult struct {
	Cost       characters.CostCommitResult
	Target     AggroTarget
	MoveResult combat.SkillMoveResult
	Counter    combat.CounterResult
	Executed   bool
	OnCooldown bool
	Crafting   bool
	NoTarget   bool
	// NotSmoke is true when the species does not list smother in
	// beast_moves, or the body has hands. Reachable only through a
	// combatcommands or btree dispatch; CanUseSmother gates the AI path.
	NotSmoke        bool
	InterruptedCast bool
}

// ExecuteSmother is the smoke elemental's special move (Appendix A of spec
// 2026-10-16 #541), mob-only: smoke pours into the target's eyes and lungs.
// It is built on ExecuteThrottle: the same cooldown, cost and skill-move
// seam; on a hit the target is Blinded (condition 3) and Smothered (144),
// and a casting target faces the shared choke contest (chokeCastInterrupt),
// at most once per special-move cooldown window. A partial or a miss applies
// nothing. No bleed: smoke has no fangs.
func ExecuteSmother(actor Actor) SmotherResult {
	char := actor.GetCharacter()

	if char.IsActing() {
		return SmotherResult{Crafting: true}
	}
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return SmotherResult{NoTarget: true}
	}
	if char.HasBodyPart("hands") || !combat.SpeciesHasBeastMove(char, "smother") {
		return SmotherResult{NotSmoke: true}
	}

	cfg := configs.GetBalanceConfig()
	if !SpecialMoveReady(char) {
		return SmotherResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionSmother, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return SmotherResult{Cost: cost}
	}
	if !ClaimSpecialMove(char) {
		return SmotherResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

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
		DamagePercent:   float64(cfg.KickDamagePercent),
		KnockdownFactor: 0, // no knockdown: blindness and choking instead
		DamageStat:      char.Stats.Strength.ValueAdj,
	})
	counter := counterSkillMoveExit(actor, target.Char, result, combatvocab.Melee(combatvocab.TargetSingle), true)

	interrupted := false
	if result.Hit {
		// Both apply synchronously, narrated by the hit line (Blinded's own
		// start lines speak of a spell; Smothered is silent-start).
		_ = target.Char.AddCondition(perception.ConditionIdBlinded, false)
		_ = target.Char.AddCondition(conditions.ConditionIdSmothered, false)
		interrupted = chokeCastInterrupt(actor, char, target.Char, sightRoom)
	}

	sourceType := combat.User
	if !actor.IsPlayer() {
		sourceType = combat.Mob
	}
	targetType := combat.User
	if target.MobInstanceId > 0 {
		targetType = combat.Mob
	}
	combat.RecordSpecialMove(sourceType, targetType, "smother", result.Hit, result.Damage, char, target.Char, util.GetRoundCount())

	// A special move consumes its round with SpendRoundOnAction, which the
	// round hook consumes silently (no wait line).
	if char.CombatPhase != nil {
		char.SpendRoundOnAction()
	}
	actor.AwardResolved(result.Hit, actor.GetCharacter().CandidateFor(string(skills.UnarmedCombat)))

	return SmotherResult{
		Cost:            cost,
		Target:          target,
		MoveResult:      result,
		Counter:         counter,
		Executed:        true,
		InterruptedCast: interrupted,
	}
}
