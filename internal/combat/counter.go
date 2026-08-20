package combat

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
)

// CounterResult holds the outcome of one counter tier firing: whether the
// tier fired at all, the seam-resolved counter-swing, and generic narration
// for the three audiences (Task 11 replaces these with per-channel counter
// triads; until then the swing reuses the shared damage-description
// vocabulary).
type CounterResult struct {
	// Countered reports whether the counter-swing actually fired. False when
	// the reach gate refused (cross-room), the knob disabled the tier, or a
	// participant was missing/dead.
	Countered bool

	// Channel is the channel of the ORIGINAL attack that was crit-defended,
	// kept for Task 11's channel-correct counter narration.
	Channel AttackChannel

	// Move is the counter-swing's full seam outcome. It always carries
	// IsCounter: the countered party defends this swing (and is charged and
	// progressed for that defence), but its result can never re-enter the
	// counter tier.
	Move SkillMoveResult

	// Damage is the health damage the counter-swing actually applied to the
	// countered party (0 when their defence stopped it).
	Damage      int
	TargetMaxHP int

	// Generic Task 10 narration. DefenderMsg addresses the COUNTERER (the one
	// who earned the counter), AttackerMsg the countered original attacker.
	DefenderMsg string
	AttackerMsg string
	RoomMsg     string
}

// ExecuteCounter fires the counter tier for a defensive crit on a
// seam-resolved channel: one free counter-swing at CounterDamagePercent of
// weapon damage (riposte's mechanism — melee's parry-crit riposte in
// internal/hooks/combat_shared_helpers.go reads the same knob, but stays on
// its historical uncontested maths so melee behaviour is unchanged).
//
// Rules, all owner decisions 2026-08-19:
//
//   - reach-gated: attacker and defender must share a room. The cross-room
//     shot is the one uncounterable attack, as a property of the weapon.
//   - defy crits COUNTER-TAUNT instead, replacing the swing. NOTE THE
//     PLACEMENT: taunt resolution lives in internal/actions, which IMPORTS
//     internal/combat — this package can never call it. The counter-taunt is
//     wired AT THE TAUNT CALL SITE in internal/actions (the defy-crit exit)
//     via a dedicated cost-free entry point that never calls this function.
//   - a counter never earns a counter: the swing goes through the seam with
//     IsCounter, and no exit fires the tier from a result produced under
//     IsCounter. ExecuteCounter itself never re-enters the tier.
//   - free for the counterer: no cost, no cooldown, like riposte today. The
//     COUNTERED party is a different story: routing the swing through the
//     seam means the original attacker defends it, and that defence is
//     charged and progressed exactly like any other (the countered-party
//     economy).
//   - this is not an interrupt — the original attack has already resolved. A
//     defensive crit is a decisive defence that leaves an opening; the
//     counter is what you do with the opening.
//
// CounterDamagePercent 0 is the documented off-switch. It is handled HERE and
// must never be forwarded into the damage pipeline: CalcRawDamage treats
// itemMult <= 0 as "unset" and substitutes 0.30, which would turn the
// off-switch into a 30%-damage counter.
func ExecuteCounter(defender, attacker *characters.Character, channel AttackChannel, sameRoom bool) CounterResult {
	result := CounterResult{Channel: channel}

	if defender == nil || attacker == nil {
		return result
	}
	// Reach gate: the cross-room shot is the one uncounterable attack.
	if !sameRoom {
		return result
	}
	// Neither a corpse nor a downed combatant answers with a counter.
	if defender.Health < 1 || attacker.Health < 1 {
		return result
	}
	pct := float64(configs.GetBalanceConfig().CounterDamagePercent)
	if pct <= 0 {
		return result
	}

	// The counter-swing, through the seam: a melee-shaped physical answer
	// (strength + the counterer's own combat skill) at CounterDamagePercent
	// of weapon damage, marked IsCounter so no exit can chain another counter
	// off it. No knockdown rider: the opening buys a strike, not a takedown —
	// dedicated followups (auto-trip/auto-bash) remain melee-crit-only.
	move := ExecuteSkillMove(SkillMoveParams{
		Attacker: defender,
		Defender: attacker,
		Channel:  ChannelMelee,
		Attack: AttackSide{
			Stat: defender.Stats.Strength.ValueAdj, StatName: "strength",
			Skill:     defender.GetCombatSkillTag(),
			SkillRank: defender.GetCombatSkillLevel(),
			Mult:      1.0,
		},
		IsCounter:       true,
		DamagePercent:   pct,
		KnockdownChance: 0,
		DamageStat:      defender.Stats.Strength.ValueAdj,
	})

	result.Countered = true
	result.Move = move
	result.Damage = move.Damage
	result.TargetMaxHP = move.TargetMaxHP
	fillCounterMessages(&result, defender, attacker)
	return result
}

// fillCounterMessages writes the generic Task 10 narration. The framing is
// deliberate: the defence already decided the attack; the counter is what the
// defender does with the opening it left. Task 11 replaces these with
// channel-correct counter triads.
func fillCounterMessages(result *CounterResult, defender, attacker *characters.Character) {
	if result.Damage > 0 {
		dmgDesc := GetDamageDescription(result.Damage, result.TargetMaxHP)
		result.DefenderMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ COUNTER!</ansi> Your decisive defense leaves an opening and you strike back at %s! (<ansi fg="damage">%s</ansi>)`,
			attacker.Name, dmgDesc)
		result.AttackerMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ COUNTER!</ansi> %s turns your failed attack into a swift strike of their own! (<ansi fg="damage">%s</ansi>)`,
			defender.Name, dmgDesc)
		result.RoomMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ COUNTER!</ansi> %s seizes the opening and strikes back at %s!`,
			defender.Name, attacker.Name)
		return
	}
	result.DefenderMsg = fmt.Sprintf(
		`<ansi fg="cyan-bold">⚔ COUNTER!</ansi> You strike back at %s, but the answer is turned aside!`,
		attacker.Name)
	result.AttackerMsg = fmt.Sprintf(
		`<ansi fg="cyan-bold">⚔ COUNTER!</ansi> %s strikes back at you, but you turn the answer aside!`,
		defender.Name)
	result.RoomMsg = fmt.Sprintf(
		`<ansi fg="cyan-bold">⚔ COUNTER!</ansi> %s strikes back at %s, but the answer is turned aside!`,
		defender.Name, attacker.Name)
}
