package combat

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// CounterResult holds the outcome of one counter tier firing: whether the
// tier fired at all, the seam-resolved counter-swing, and channel-correct
// narration for the three audiences (U6b Task 11: rendered from the
// counter-* pools in defense-messages/, chosen by the ORIGINAL attack's
// channel).
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

	// CountererUserId is the counterer's user id (0 for mobs), captured so
	// dispatchers that only receive this result (the Task 11 ordering fix
	// returns it up through the action result structs) can route the
	// counterer's private line without re-resolving the character.
	CountererUserId int

	// Channel-correct counter narration (U6b Task 11), rendered from the
	// counter-* pools in _datafiles/world/dogmud/defense-messages/.
	// DefenderMsg addresses the COUNTERER (the one who earned the counter),
	// AttackerMsg the countered original attacker.
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
		KnockdownFactor: 0,
		DamageStat:      defender.Stats.Strength.ValueAdj,
	})

	result.Countered = true
	result.Move = move
	result.Damage = move.Damage
	result.TargetMaxHP = move.TargetMaxHP
	result.CountererUserId = defender.GetUserId()
	fillCounterMessages(&result, defender, attacker)
	return result
}

// counterPrefix marks every counter line so the tier stays scannable in a
// busy round; the narration itself comes from the channel's counter pool.
const counterPrefix = `<ansi fg="cyan-bold">⚔ COUNTER!</ansi> `

// counterPoolFor maps the ORIGINAL attack's channel to its counter-narration
// pool (U6b Task 11): a counter must read channel-correct, never a generic
// riposte string pasted under a spell. Both spell channels share the
// put-the-working-down pool.
//
// ChannelSocial used to be unreachable here, because taunt is the only social
// attack and it short-circuits its defy-crit into a counter-TAUNT at the call
// site rather than swinging. U10c broke that assumption: charm is now a social
// SPELL, and fireSpellCounterTier has no such carve-out, so a defy-crit against
// a charm does arrive.
//
// It gets counter-defy rather than falling through to the physical pool. That
// pool already exists and was unused on this path. Note the counter itself is
// still a physical swing -- ExecuteCounter builds strength + combat skill for
// every channel, and the channel selects narration only -- so this makes the
// prose honest, not the mechanics social. Giving social attacks a genuinely
// social counter is a larger change than U10c's plumbing slice.
func counterPoolFor(channel AttackChannel) items.DefenseType {
	switch channel {
	case ChannelRanged:
		return items.DefenseCounterRanged
	case ChannelSpellPhysical, ChannelSpellMental:
		return items.DefenseCounterQuell
	case ChannelSocial:
		return items.DefenseCounterDefy
	default:
		return items.DefenseCounterMelee
	}
}

// counterBand converts a counter outcome to the pool's band inputs: heavy
// (crit=true) when the counter-swing itself critted and landed, normal
// (margin 1.0) when it landed, weak otherwise (turned aside or fumbled).
func counterBand(crit bool, damage int) (bandCrit bool, bandMargin float64) {
	if damage <= 0 {
		return false, 0.0
	}
	return crit, 1.0
}

// fillCounterMessages renders the channel-correct counter triad (U6b Task 11)
// from the counter-* pools, appending the damage description to the two
// personal lines the same way the special-move wrappers do (room lines never
// carry damage). The framing is deliberate: the defence already decided the
// attack; the counter is what the defender does with the opening it left.
// When the pool is not loaded (unit tests without data files), the generic
// Task 10 narration stands in so the tier never goes silent.
func fillCounterMessages(result *CounterResult, defender, attacker *characters.Character) {
	bandCrit, bandMargin := counterBand(result.Move.Crit, result.Damage)
	triad := items.RenderDefenseMessage(counterPoolFor(result.Channel), bandCrit, bandMargin,
		map[items.TokenName]string{
			items.TokenAttacker: attacker.Name,
			items.TokenDefender: defender.Name,
		})
	if triad.ToRoom == "" {
		fillGenericCounterMessages(result, defender, attacker)
		return
	}
	dmgTag := ""
	if result.Damage > 0 {
		dmgTag = fmt.Sprintf(` (<ansi fg="damage">%s</ansi>)`,
			GetDamageDescription(result.Damage, result.TargetMaxHP))
	}
	result.DefenderMsg = counterPrefix + string(triad.ToDefender) + dmgTag
	result.AttackerMsg = counterPrefix + string(triad.ToAttacker) + dmgTag
	result.RoomMsg = counterPrefix + string(triad.ToRoom)
}

// fillGenericCounterMessages is the Task 10 narration, kept only as the
// fallback for environments where the counter pools are not loaded.
func fillGenericCounterMessages(result *CounterResult, defender, attacker *characters.Character) {
	if result.Damage > 0 {
		dmgDesc := GetDamageDescription(result.Damage, result.TargetMaxHP)
		result.DefenderMsg = fmt.Sprintf(
			counterPrefix+`Your decisive defense leaves an opening and you strike back at %s! (<ansi fg="damage">%s</ansi>)`,
			attacker.Name, dmgDesc)
		result.AttackerMsg = fmt.Sprintf(
			counterPrefix+`%s turns your failed attack into a swift strike of their own! (<ansi fg="damage">%s</ansi>)`,
			defender.Name, dmgDesc)
		result.RoomMsg = fmt.Sprintf(
			counterPrefix+`%s seizes the opening and strikes back at %s!`,
			defender.Name, attacker.Name)
		return
	}
	result.DefenderMsg = fmt.Sprintf(
		counterPrefix+`You strike back at %s, but the answer is turned aside!`,
		attacker.Name)
	result.AttackerMsg = fmt.Sprintf(
		counterPrefix+`%s strikes back at you, but you turn the answer aside!`,
		defender.Name)
	result.RoomMsg = fmt.Sprintf(
		counterPrefix+`%s strikes back at %s, but the answer is turned aside!`,
		defender.Name, attacker.Name)
}

// retortPrefix marks the defy carve-out's counter-taunt lines.
const retortPrefix = `<ansi fg="cyan-bold">⚔ RETORT!</ansi> `

// BuildCounterTauntMessages renders the defy counter-taunt triad (U6b Task 11)
// from the counter-defy pool: the jeer turned back on the one who threw it.
// countererName is the one whose defy critted; taunterName the original
// taunter now being counter-taunted. Damage is conviction damage; the
// description is appended to the two personal lines only. Lives here (not in
// internal/actions with the carve-out's wiring) so every counter narration
// composes through the same pool idiom; falls back to the generic Task 10
// retort lines when the pool is not loaded.
func BuildCounterTauntMessages(countererName, taunterName string, crit bool, damage, taunterMaxCP int) (countererMsg, taunterMsg, roomMsg string) {
	bandCrit, bandMargin := counterBand(crit, damage)
	triad := items.RenderDefenseMessage(items.DefenseCounterDefy, bandCrit, bandMargin,
		map[items.TokenName]string{
			items.TokenAttacker: taunterName,
			items.TokenDefender: countererName,
		})
	if triad.ToRoom == "" {
		return buildGenericCounterTauntMessages(countererName, taunterName, damage, taunterMaxCP)
	}
	dmgTag := ""
	if damage > 0 {
		dmgTag = fmt.Sprintf(` (<ansi fg="damage">%s</ansi>)`,
			GetConvictionDamageDescription(damage, taunterMaxCP))
	}
	return retortPrefix + string(triad.ToDefender) + dmgTag,
		retortPrefix + string(triad.ToAttacker) + dmgTag,
		retortPrefix + string(triad.ToRoom)
}

// buildGenericCounterTauntMessages is the Task 10 retort narration, kept only
// as the fallback for environments where the counter-defy pool is not loaded.
func buildGenericCounterTauntMessages(countererName, taunterName string, damage, taunterMaxCP int) (countererMsg, taunterMsg, roomMsg string) {
	if damage > 0 {
		dmgDesc := GetConvictionDamageDescription(damage, taunterMaxCP)
		return fmt.Sprintf(retortPrefix+`You throw %s's taunt right back in their face! (<ansi fg="damage">%s</ansi>)`, taunterName, dmgDesc),
			fmt.Sprintf(retortPrefix+`%s throws your taunt right back in your face! (<ansi fg="damage">%s</ansi>)`, countererName, dmgDesc),
			fmt.Sprintf(retortPrefix+`%s throws %s's taunt right back!`, countererName, taunterName)
	}
	return fmt.Sprintf(retortPrefix+`You snap back at %s, but the words fail to bite!`, taunterName),
		fmt.Sprintf(retortPrefix+`%s snaps back at you, but the words fail to bite!`, countererName),
		fmt.Sprintf(retortPrefix+`%s snaps back at %s!`, countererName, taunterName)
}
