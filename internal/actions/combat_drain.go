package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// DrainResult holds the outcome of a drain attempt for the caller to use when
// formatting messages, firing events, and updating UI.
type DrainResult struct {
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

	// Executed reports whether the drain was actually performed. False when any
	// early-exit condition fired (OnCooldown, NoTarget, NotLifeDrainer).
	Executed bool

	// OnCooldown is true when the special-move cooldown blocked the drain.
	OnCooldown bool

	// Crafting is true when the actor is occupied by another activity.
	Crafting bool

	// NoTarget is true when there is no aggro target or the target is gone.
	NoTarget bool

	// NotLifeDrainer is true when the actor's species lacks the LifeDrain flag.
	// Reachable via a direct player command or a btree/combatcommands dispatch
	// to a non-lifedrain mob. Unreachable via the AI path (CanUseDrain gates it).
	NotLifeDrainer bool

	// Healed is the amount of HP the attacker actually recovered from the
	// lifesteal (after clamping to HealthMax). Scales on damage actually
	// dealt, so it is nonzero on a partial-damage defended attempt too. Zero
	// only when no damage was dealt (defensive crit or a fully-avoided move).
	Healed int

	// BleedDmg is the per-round amount of the bleed stack added to the
	// target on a hit.
	BleedDmg int
}

// ExecuteDrain performs the core drain resolution shared between player and mob
// callers. It handles:
//   - Special-move cooldown (using SpecialMoveCooldown from balance config)
//   - Target resolution via ResolveAggroTarget
//   - Species identity gate: SpeciesHasLifeDrain
//   - ExecuteSkillMove via combat package (UnarmedCombat skill, Strength
//     attack stat, Dexterity defense stat, TripDamagePercent, Strength damage
//     stat, no knockdown)
//   - On hit: add a Bleeding stack (DrainBleedRounds, DrainBleedStrengthDivisor,
//     DrainBleedMin) sourced as "drain". Lifesteal heals the attacker for
//     DrainHealRatio * damage actually dealt, including a defended attempt
//     that still lands partial damage (bleed stays hit-only; lifesteal does
//     not)
//   - combat.RecordSpecialMove for analytics + RoundsWaiting = 1
//   - OnSkillUse(UnarmedCombat) on hit for progression
//
// Callers are responsible for all messaging and any combat-initiation logic.
func ExecuteDrain(actor Actor) DrainResult {
	char := actor.GetCharacter()

	if char.IsActing() {
		return DrainResult{Crafting: true}
	}

	// Resolve the aggro target.
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return DrainResult{NoTarget: true}
	}

	// Identity gate (defense-in-depth): only LifeDrain species can drain.
	// Unreachable via the AI path (CanUseDrain gates it) but reachable via a
	// direct player command or a btree/combatcommands dispatch to a non-lifedrain
	// mob.
	if !combat.SpeciesHasLifeDrain(char) {
		return DrainResult{NotLifeDrainer: true}
	}

	cfg := configs.GetBalanceConfig()
	if !SpecialMoveReady(char) {
		return DrainResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionDrain, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return DrainResult{Cost: cost}
	}
	if !ClaimSpecialMove(char) {
		return DrainResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

	// Execute the skill move. Drain uses TripDamagePercent — the sapping strike
	// is lighter than a full melee blow; the lifesteal makes up the difference.
	// Strength drives both the attack and the damage, reflecting the predatory
	// grip. Dexterity governs the defender's evasion.
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
			Stat: char.Stats.Strength.ValueAdj, StatName: "strength",
			Skill: skills.UnarmedCombat, SkillRank: char.GetSkillLevel(skills.UnarmedCombat),
			Mult:      combat.SituationalAttackMult(char, sightRoom, combatvocab.Melee(combatvocab.TargetSingle)),
			ForceCrit: combat.SleepingForceCrit(target.Char),
		},
		DamagePercent:   float64(cfg.TripDamagePercent),
		KnockdownFactor: 0, // No knockdown — the drain itself is the payoff
		DamageStat:      char.Stats.Strength.ValueAdj,
	})

	// U6b Task 10: a crit-defended move earns the defender a counter-swing.
	counter := counterSkillMoveExit(actor, target.Char, result, combatvocab.Melee(combatvocab.TargetSingle), true)

	// On hit: bleed the victim. Bleed is a status effect (binary), so it stays
	// gated on a clean hit. One stack: DrainBleedRounds rounds, Strength /
	// DrainBleedStrengthDivisor per round, floor DrainBleedMin.
	bleedDmg := 0
	if result.Hit {
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.DrainBleedStrengthDivisor, cfg.DrainBleedMin)
		_ = target.Char.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, int(cfg.DrainBleedRounds), -float64(bleedDmg), "drain", ActorRefOf(actor))
	}

	// Lifesteal: heal the attacker for a fraction of damage dealt. Gated on
	// damage actually applied rather than on Hit, per U6's shared partial
	// rule: anything that scales on damage (reflect, lifesteal, on-hit procs)
	// reads the damage actually dealt, so a defended drain that still clips
	// the target for partial damage still feeds the attacker.
	healed := 0
	if result.Damage > 0 {
		healAmt := int(float64(result.Damage) * float64(cfg.DrainHealRatio))
		if healAmt < 1 {
			healAmt = 1
		}
		healed = char.Heal(healAmt)
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
	combat.RecordSpecialMove(sourceType, targetType, "drain", result.Hit, dmgRecorded, char, target.Char, util.GetRoundCount())

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

	return DrainResult{
		Cost:       cost,
		Target:     target,
		MoveResult: result,
		Counter:    counter,
		Executed:   true,
		Healed:     healed,
		BleedDmg:   bleedDmg,
	}
}

// DrainAreaPlayerResult captures the per-player outcome of a single player
// target within an ExecuteDrainArea sweep.
type DrainAreaPlayerResult struct {
	// UserId identifies which player this result belongs to.
	UserId int

	// MoveResult is the outcome from ExecuteSkillMove against this player.
	MoveResult combat.SkillMoveResult

	// BleedDmg is the per-round amount of the bleed stack added to this
	// player on a hit. Zero on a miss.
	BleedDmg int
}

// DrainAreaResult holds the aggregate outcome of an ExecuteDrainArea sweep.
type DrainAreaResult struct {
	// Executed reports whether the area drain actually ran (false only when
	// there was no room, or no living players in it, to drain).
	Executed bool

	// NoTargets is true when the actor has no room, or the room has no
	// living players to drain.
	NoTargets bool

	// PlayerResults holds the per-player outcome for every player that was
	// swept by the drain (hit or miss).
	PlayerResults []DrainAreaPlayerResult

	// TotalDamage is the sum of damage actually applied across all swept
	// players, including partial damage from players who defended the move.
	TotalDamage int

	// Healed is the amount of HP the actor actually recovered (after
	// clamping to HealthMax) from the aggregate lifesteal across every player
	// who took damage (hit or partial). Zero if no player took any damage.
	Healed int
}

// ExecuteDrainArea performs an area version of ExecuteDrain: every living
// player in the actor's room is swept for drain damage (sharing
// ExecuteDrain's TripDamagePercent / bleed-on-hit / DrainHealRatio lifesteal
// math, but NOT its defence set — see the Shape note in the loop below), and
// the actor is healed once by the aggregate of
// every damaging result's lifesteal fraction (hit or defended partial;
// bleed stays hit-only). Summing per-result heal fractions and healing once
// at the end is mathematically identical to computing the aggregate fraction
// of the total damage up front (both reduce to DrainHealRatio *
// sum(damage_i)) — the per-result accumulation is what lets this loop also
// report per-player damage for messaging.
//
// Unlike ExecuteDrain, this is NOT gated by SpeciesHasLifeDrain / aggro / the
// special-move cooldown — those are defense-in-depth for the single-target
// player/mob "drain" command entry point. ExecuteDrainArea is a boss-ability
// primitive meant to be invoked once, at the moment a fold-cast spell
// resolves (see resolveMobDrainArea in internal/hooks/spell_resolution.go),
// by a caster (e.g. a construct boss) that has no reason to carry the
// vampire-specific LifeDrain species flag. The caller (the spell-effect
// dispatch) owns cast-completion timing and telegraph/interrupt semantics;
// this function only owns the room-wide damage/heal math.
//
// Callers are responsible for all messaging.
func ExecuteDrainArea(actor Actor) DrainAreaResult {
	char := actor.GetCharacter()
	room := actor.GetRoom()
	if room == nil {
		return DrainAreaResult{NoTargets: true}
	}

	cfg := configs.GetBalanceConfig()
	playerIds := room.GetPlayers(rooms.FindAll)

	// M4b-2 (owner ruling, M4 spec 9): core-drain is a PHYSICAL SPELL. Its
	// victims dodge or block; nobody parries a room. This also drops the
	// melee prone/stamina accuracy penalty, because a cast pays neither.
	// A balance change, in its own commit. A crit-defended drain earns no
	// counter: the drain is an area attack (counters slice).

	result := DrainAreaResult{}
	totalHeal := 0
	for _, uid := range playerIds {
		target := users.GetByUserId(uid)
		if target == nil || target.Character.Health < 1 {
			continue // skip vanished/downed players — already out of the fight
		}

		// U6b Task 7: through the channel seam, same conversion as the
		// single-target drain above — each player defends with their own
		// equipment-gated set, charged and progressed per contest.
		moveResult := combat.ExecuteSkillMove(combat.SkillMoveParams{
			Attacker: char,
			Defender: target.Character,
			Shape:    combatvocab.Spell(combatvocab.DamagePhysical, combatvocab.TargetArea),
			Room:     room, // lighting plan 5b: non-nil, checked above
			Attack: combat.AttackSide{
				Stat: char.Stats.Strength.ValueAdj, StatName: "strength",
				Skill: skills.UnarmedCombat, SkillRank: char.GetSkillLevel(skills.UnarmedCombat),
				Mult:      combat.SituationalAttackMult(char, room, combatvocab.Spell(combatvocab.DamagePhysical, combatvocab.TargetArea)),
				ForceCrit: combat.SleepingForceCrit(target.Character),
			},
			DamagePercent: float64(cfg.TripDamagePercent),
			DamageStat:    char.Stats.Strength.ValueAdj,
		})

		// Counters slice: a room-wide drain is an AREA attack and earns no
		// counter, and the primitive would refuse one. The exit is not
		// called rather than called into a gate that always refuses.
		pr := DrainAreaPlayerResult{UserId: uid, MoveResult: moveResult}

		// Bleed is a status effect (binary), so it stays gated on a clean hit.
		if moveResult.Hit {
			pr.BleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.DrainBleedStrengthDivisor, cfg.DrainBleedMin)
			_ = target.Character.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, int(cfg.DrainBleedRounds), -float64(pr.BleedDmg), "drain", ActorRefOf(actor))
		}

		// Lifesteal reads the damage actually applied, per U6's shared partial
		// rule, so a defended drain that still clips a player still feeds the
		// aggregate heal.
		if moveResult.Damage > 0 {
			healAmt := int(float64(moveResult.Damage) * float64(cfg.DrainHealRatio))
			if healAmt < 1 {
				healAmt = 1
			}
			totalHeal += healAmt
			result.TotalDamage += moveResult.Damage
		}

		result.PlayerResults = append(result.PlayerResults, pr)
	}

	if len(result.PlayerResults) == 0 {
		return DrainAreaResult{NoTargets: true}
	}

	result.Executed = true
	if totalHeal > 0 {
		result.Healed = char.Heal(totalHeal)
	}

	return result
}
