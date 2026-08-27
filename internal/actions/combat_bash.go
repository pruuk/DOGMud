package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// BashResult holds the outcome of a bash attempt for the caller to use when
// formatting messages, firing events, and updating UI.
type BashResult struct {
	Cost characters.CostCommitResult

	// Target is the resolved aggro target. Valid only when Executed is true.
	Target AggroTarget

	// MoveResult is the outcome from ExecuteSkillMove. Valid only when Executed is true.
	MoveResult combat.SkillMoveResult

	// Counter is the counter tier outcome (U6b Tasks 10-11): non-zero when the
	// defender crit-defended and answered. The command wrapper speaks its
	// narration AFTER the move's own outcome via DispatchCounterMessages.
	Counter combat.CounterResult

	// Executed reports whether the bash was actually performed. False when any
	// early-exit condition fired (OnCooldown, NoTarget, NoShield).
	Executed bool

	// OnCooldown is true when the special-move cooldown blocked the bash.
	OnCooldown bool

	// Crafting is true when the actor is mid-craft and cannot bash.
	Crafting bool

	// NoTarget is true when there is no aggro target or the target is gone.
	NoTarget bool

	// NoShield is true when the actor has no shield equipped.
	NoShield bool
}

// ExecuteBash performs the core bash resolution shared between player and mob
// callers. It handles:
//   - Shield check
//   - Special-move cooldown (using SpecialMoveCooldown from balance config)
//   - Target resolution via ResolveAggroTarget
//   - ExecuteSkillMove via combat package
//   - RecordAndWait (analytics + round consumption)
//
// Callers are responsible for all messaging, skill progression events, and
// any combat-initiation logic (e.g. SetAggro for player out-of-combat bash).
func ExecuteBash(actor Actor) BashResult {
	char := actor.GetCharacter()

	// Don't interrupt any active activity (cast/craft/salvage) to swing a shield.
	if char.IsActing() {
		return BashResult{Crafting: true}
	}

	// Must have a shield equipped — unless this creature bashes naturally
	// (golems/elementals slam with their body).
	naturalBash := false
	if sp := species.GetSpecies(char.SpeciesId); sp != nil {
		naturalBash = sp.NaturalBash
	}
	if !char.HasShield() && !naturalBash {
		return BashResult{NoShield: true}
	}
	// Defense-in-depth: anatomy gate. Unreachable for players (always armed);
	// AI/readiness gates already block no-arms mobs upstream. Reuse NoShield
	// rather than add a flag for an unreachable branch.
	if !char.HasBodyPart("arms") && !naturalBash {
		return BashResult{NoShield: true}
	}

	// Resolve the aggro target.
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return BashResult{NoTarget: true}
	}

	cfg := configs.GetBalanceConfig()
	if !char.CooldownReady("special-move") {
		return BashResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionBash, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return BashResult{Cost: cost}
	}
	if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return BashResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

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
			Skill: skills.WeaponCombat, SkillRank: char.GetSkillLevel(skills.WeaponCombat),
			Mult:      combat.SituationalAttackMult(char, combat.ChannelMelee),
			ForceCrit: combat.SleepingForceCrit(target.Char),
		},
		DamagePercent:     float64(cfg.BashDamagePercent),
		KnockdownFactor:   float64(cfg.BashKnockdownFactor),
		DamageStat:        char.Stats.Strength.ValueAdj,
		KnockdownToSupine: true, // bash sends defender backward
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

	// Record analytics and consume the combat round. result.Damage is always
	// the amount actually applied (hit, partial-on-defended, or 0 on a
	// defensive crit), so it is truthful whether or not the move landed.
	dmgRecorded := result.Damage
	RecordAndWait(char, "bash", sourceType, target.Char, targetType, result.Hit, dmgRecorded, util.GetRoundCount())

	// Progression: weapon-combat on hit (moved from user/mob wrappers)
	// U10b-1 Task 18b: win OR lose. This was gated on the hit, so a special
	// move that missed trained nothing -- the same defect a failed craft had
	// before Task 16. The gate is now the AWARD WEIGHT rather than a
	// precondition; a thrown move is a resolved contest either way.
	actor.AwardResolved(result.Hit, actor.GetCharacter().CandidateFor(string(skills.WeaponCombat)))

	return BashResult{
		Cost:       cost,
		Target:     target,
		MoveResult: result,
		Counter:    counter,
		Executed:   true,
	}
}
