package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// ThrottleResult holds the outcome of a throttle attempt for the caller to use
// when formatting messages, firing events, and updating UI.
type ThrottleResult struct {
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

	// Executed reports whether the throttle was actually performed. False when
	// any early-exit condition fired (OnCooldown, NoTarget).
	Executed bool

	// OnCooldown is true when the special-move cooldown blocked the throttle.
	OnCooldown bool

	// Crafting is true when the actor is occupied by another activity.
	Crafting bool

	// NoTarget is true when there is no aggro target or the target is gone.
	NoTarget bool

	// NotFanged is true when the actor's species lacks a fanged natural attack.
	// Reachable via a direct player command or a btree/combatcommands dispatch
	// to a non-fanged mob. Unreachable via the AI path (CanUseThrottle gates it).
	NotFanged bool

	// InterruptedCast is true when the hit triggered the cast-cancel path and
	// the target's spellcast was successfully interrupted.
	InterruptedCast bool

	// BleedDmg is the per-tick bleed magnitude applied on a hit
	// (Strength/10, min 2).
	BleedDmg int
}

// ExecuteThrottle performs the core throttle resolution shared between player
// and mob callers. It handles:
//   - Special-move cooldown (using SpecialMoveCooldown from balance config)
//   - Target resolution via ResolveAggroTarget
//   - ExecuteSkillMove via combat package (UnarmedCombat skill, Dexterity
//     attack stat, Dexterity defense stat, KickDamagePercent, Strength damage
//     stat, no knockdown)
//   - On hit: apply ConditionBleeding (duration 3, magnitude = Strength/10
//     min 2) sourced as "throttle"
//   - On hit: apply Throttled DoT buff (id 89) for stamina drain
//   - On hit: an opposed contest through the concentration seam
//     (combat.RunConcentrationContest) between the target's hold and the
//     throttler's grip; a lost hold interrupts via InterruptTargetCast
//     (engine's standard cast-cancel path)
//   - combat.RecordSpecialMove for analytics + RoundsWaiting = 1
//   - OnSkillUse(UnarmedCombat) on hit for progression
//
// Callers are responsible for all messaging and any combat-initiation logic.
func ExecuteThrottle(actor Actor) ThrottleResult {
	char := actor.GetCharacter()

	if char.IsActing() {
		return ThrottleResult{Crafting: true}
	}

	// Resolve the aggro target.
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return ThrottleResult{NoTarget: true}
	}

	// Anatomy/identity gate (defense-in-depth): only handless fanged creatures
	// throttle. Unreachable via the AI path (CanUseThrottle gates it) but
	// reachable via a direct player command or a btree/combatcommands dispatch
	// to a non-fanged or tool-using mob.
	if char.HasBodyPart("hands") || !combat.SpeciesIsFanged(char) {
		return ThrottleResult{NotFanged: true}
	}

	cfg := configs.GetBalanceConfig()
	if !char.CooldownReady("special-move") {
		return ThrottleResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionThrottle, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return ThrottleResult{Cost: cost}
	}
	if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return ThrottleResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

	// Execute the skill move (reuse kick's config for damage percent; no
	// knockdown — the choke deals stamina drain and cast interrupt instead).
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
		DamagePercent:   float64(cfg.KickDamagePercent),
		KnockdownFactor: 0, // No knockdown — choke + stamina drain instead
		DamageStat:      char.Stats.Strength.ValueAdj,
	})

	// U6b Task 10: a crit-defended move earns the defender a counter-swing.
	counter := counterSkillMoveExit(actor, target.Char, result, combat.ChannelMelee, true)

	bleedDmg := 0
	interrupted := false

	if result.Hit {
		// Health-over-time: apply a light bleed (duration 3, magnitude = Strength/10,
		// min 2) — weaker than maul; the choke's primary DoT is stamina drain.
		mag := char.Stats.Strength.ValueAdj / 10
		if mag < 2 {
			mag = 2
		}
		target.Char.AddCondition(characters.ConditionBleeding, 3, float64(mag), "throttle")
		bleedDmg = mag

		// Stamina-over-time: apply the Throttled DoT buff (id 89).
		_ = target.Char.AddBuff(89, false)

		// Cast interrupt (U10): an opposed contest through the
		// concentration seam — the caster's hold (attack side, as in every
		// concentration contest) against the throttler's grip, floored at
		// ConcentrationFloor per the owner's 2% ruling. Telegraphed
		// NoDamageInterrupt casts are not contestable, matching both
		// concentration paths (deliberate small fix: the old flat coin
		// ignored the flag; throttle is mob-only by anatomy, so no player
		// counter-play is lost).
		if target.Char.IsCasting() {
			interruptible := true
			if cs, ok := target.Char.Activity.CastingData(); ok {
				if sd := spells.GetSpell(cs.SpellId); sd != nil && sd.NoDamageInterrupt {
					interruptible = false
				}
			}
			if interruptible {
				w := float64(cfg.SkillWeight)
				grip := float64(char.Stats.Dexterity.ValueAdj) +
					float64(char.GetSkillLevel(skills.UnarmedCombat))*w
				hold := float64(target.Char.Stats.Willpower.ValueAdj) +
					float64(target.Char.GetSkillLevel(skills.Spellcasting))*w
				res := combat.RunConcentrationContest(hold, grip)
				// U10b-1 Task 12: the DEFENDER's concentration award, win or
				// lose, fired before the branch because the loss arm has its
				// own work to do. It was previously success-only and sat in
				// the else. res.Success is the caster's win -- hold is passed
				// as the attack side of the contest.
				//
				// The throttler's own progression came from the move's hit
				// resolution; this is not a second attacker event.
				target.Char.AwardResolved(
					target.Char.GetUserId(), res.Success,
					target.Char.CandidateFor(string(skills.Spellcasting)))
				if !res.Success {
					var attackerRef state.ActorRef
					if actor.IsPlayer() {
						attackerRef = state.ActorRef{UserId: actor.GetUserId()}
					} else {
						attackerRef = state.ActorRef{MobInstanceId: actor.GetMobInstanceId()}
					}
					interrupted = InterruptTargetCast(target.Char, attackerRef)
				}
			}
		}
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
	combat.RecordSpecialMove(sourceType, targetType, "throttle", result.Hit, dmgRecorded, char, target.Char, util.GetRoundCount())

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

	return ThrottleResult{
		Cost:            cost,
		Target:          target,
		MoveResult:      result,
		Counter:         counter,
		Executed:        true,
		BleedDmg:        bleedDmg,
		InterruptedCast: interrupted,
	}
}
