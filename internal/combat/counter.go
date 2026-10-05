package combat

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// CounterResult holds the outcome of one counter tier firing: whether the
// tier fired at all, the seam-resolved counter-swing, and defence-correct
// narration for the three audiences (U6b Task 11: rendered from the
// counter-* pools in defense-messages/, chosen by the defence that won).
type CounterResult struct {
	// Countered reports whether the counter-swing actually fired. False when
	// the reach gate refused (cross-room), the attack was not single-target,
	// the winning defence was defy (the counter-taunt answers instead) or
	// none, the knob disabled the tier, or a participant was missing/dead.
	Countered bool

	// Defence is the defence that WON the original contest and earned this
	// counter. It chooses the narration pool (counters slice, spec ruling 1):
	// a parry crit reads the parry pool, a block crit the block pool.
	Defence combatvocab.Defence

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

	// CountererName and CounteredName are the plain names the narration below
	// is built from, so the dispatchers can hand them to messaging.SendTrio,
	// which hides a name from a reader who cannot see that party.
	CountererName string
	CounteredName string

	// Defence-correct counter narration (U6b Task 11), rendered from the
	// counter-* pools in _datafiles/world/dogmud/defense-messages/.
	// DefenderMsg addresses the COUNTERER (the one who earned the counter),
	// AttackerMsg the countered original attacker.
	DefenderMsg string
	AttackerMsg string
	RoomMsg     string
}

// ExecuteCounter fires the counter tier for a defensive crit on a
// seam-resolved channel, narrated from the pool of the defence that won it:
// one free counter-swing at CounterDamagePercent of weapon damage (riposte's
// mechanism — melee's parry-crit riposte in
// internal/hooks/combat_shared_helpers.go reads the same knob, but stays on
// its historical uncontested maths so melee behaviour is unchanged).
//
// Rules, all owner decisions 2026-08-19 unless dated otherwise:
//
//   - reach-gated: attacker and defender must share a room. The cross-room
//     shot is the one single-target attack that cannot be countered, as a
//     property of the weapon.
//   - single-target only: an area or multi attack earns no counter (owner
//     ruling 2026-09-18). Targeting travels on the shape, so an exit cannot
//     bypass the gate by omission.
//   - defy crits COUNTER-TAUNT instead, replacing the swing, whatever the
//     attack was (taunt or charm). NOTE THE PLACEMENT: taunt resolution
//     lives in internal/actions, which IMPORTS internal/combat; this package
//     can never call it. Every exit that can see a defy win branches to
//     internal/actions.FireCounterTaunt first, and this function refuses a
//     defy defence outright.
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
func ExecuteCounter(defender, attacker *characters.Character, shape combatvocab.Attack, defence combatvocab.Defence, sameRoom bool) CounterResult {
	result := CounterResult{Defence: defence}

	if defender == nil || attacker == nil {
		return result
	}
	// A counter is narrated by the defence that won it. No winner, no pool:
	// cannot happen today (DefensiveCrit is set only after the winner is
	// recorded, pinned by TestResolveChannelAttack_ADefensiveCritNamesItsDefence)
	// but the primitive refuses, and logs, rather than rendering an empty
	// pool; the cost of this refusal is the swing itself, not only its text.
	if defence == combatvocab.DefenceNone {
		mudlog.Warn("counter", "refused", "no winning defence recorded", "attack", shape.String())
		return result
	}
	// A counter answers one deliberate attack at one target (owner ruling,
	// counters spec 2). An area or multi attack earns none, however
	// decisively one victim turned it aside. The gate lives HERE, and its
	// twin in actions.FireCounterTaunt, so no exit can bypass it: the spell
	// exits pass the spell's authored targeting and the area spells fall
	// out; throw never had an exit, and now this says why.
	if shape.Targeting != combatvocab.TargetSingle {
		return result
	}
	// Words answer words: a defy crit counter-taunts, for charm as well as
	// for taunt (owner ruling 2026-09-18). That answer lives in
	// internal/actions.FireCounterTaunt, which this package cannot call, so
	// the primitive refuses the defence rather than swinging steel at a
	// jeer. Callers branch on the defence BEFORE reaching here.
	if defence == combatvocab.DefenceDefy {
		return result
	}
	// Reach gate: the cross-room shot is the one single-target attack that
	// cannot be countered.
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
	//
	// Lighting plan 5b: the counter-swing sees too. This function holds only
	// the characters, so the room (shared: sameRoom is true here) is loaded
	// once. The swing never composed prone or stamina (its Mult was a flat
	// 1.0); it takes the sight row alone. The seam applies the answering
	// side's sight row.
	room := SightRoom(rooms.LoadRoom(defender.RoomId))
	move := ExecuteSkillMove(SkillMoveParams{
		Attacker:   defender,
		Defender:   attacker,
		Shape:      combatvocab.Melee(combatvocab.TargetSingle),
		Room:       room,
		StrikeWith: defender.WieldedWeaponPtr(),
		Attack: AttackSide{
			Stat: defender.Stats.Strength.ValueAdj, StatName: "strength",
			Skill:     defender.GetCombatSkillTag(),
			SkillRank: defender.GetCombatSkillLevel(),
			Mult:      messaging.SightMult(defender, room),
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
	result.CountererName = defender.Name
	result.CounteredName = attacker.Name
	fillCounterMessages(&result, defender, attacker)
	return result
}

// counterPrefix marks every counter line so the tier stays scannable in a
// busy round; the narration itself comes from the winning defence's counter
// pool.
const counterPrefix = `<ansi fg="cyan-bold">⚔ COUNTER!</ansi> `

// counterBand converts a counter outcome to the pool's band inputs: heavy
// (crit=true) when the counter-swing itself critted and landed, normal
// (margin 1.0) when it landed, weak otherwise (turned aside or fumbled).
func counterBand(crit bool, damage int) (bandCrit bool, bandMargin float64) {
	if damage <= 0 {
		return false, 0.0
	}
	return crit, 1.0
}

// fillCounterMessages renders the defence-correct counter triad (U6b Task 11)
// from the winning defence's counter-* pool (items.CounterPoolFor), appending
// the damage description to the two personal lines the same way the
// special-move wrappers do (room lines never carry damage). The framing is
// deliberate: the defence already decided the attack; the counter is what
// the defender does with the opening it left. When the pool is not loaded
// (unit tests without data files), the generic Task 10 narration stands in
// so the tier never goes silent.
func fillCounterMessages(result *CounterResult, defender, attacker *characters.Character) {
	bandCrit, bandMargin := counterBand(result.Move.Crit, result.Damage)
	triad := items.RenderDefenseMessage(items.CounterPoolFor(result.Defence), bandCrit, bandMargin,
		map[items.TokenName]string{
			items.TokenActor: meleeIdentityTag(attacker),
			items.TokenActee: meleeIdentityTag(defender),
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
// counterer is the one whose defy critted; countered the one whose words
// (taunt or charm) were defied. Damage is conviction damage; the description
// is appended to the two personal lines only. Lives here (not in
// internal/actions with the carve-out's wiring) so every counter narration
// composes through the same pool idiom; falls back to the generic Task 10
// retort lines when the pool is not loaded.
//
// Task 4c: took *characters.Character rather than bare names, the same
// change fillCounterMessages got, so this counter-defy path stops being the
// one counter renderer that still substitutes a raw name into an observer
// line. It was the sibling this file's own guard test caught: identical bug,
// same file, left unfixed while its two neighbours were tagged.
func BuildCounterTauntMessages(counterer, countered *characters.Character, crit bool, damage, counteredMaxCP int) (countererMsg, taunterMsg, roomMsg string) {
	bandCrit, bandMargin := counterBand(crit, damage)
	triad := items.RenderDefenseMessage(items.CounterPoolDefy, bandCrit, bandMargin,
		map[items.TokenName]string{
			items.TokenActor: meleeIdentityTag(countered),
			items.TokenActee: meleeIdentityTag(counterer),
		})
	if triad.ToRoom == "" {
		return buildGenericCounterTauntMessages(counterer.Name, countered.Name, damage, counteredMaxCP)
	}
	dmgTag := ""
	if damage > 0 {
		dmgTag = fmt.Sprintf(` (<ansi fg="damage">%s</ansi>)`,
			GetConvictionDamageDescription(damage, counteredMaxCP))
	}
	return retortPrefix + string(triad.ToDefender) + dmgTag,
		retortPrefix + string(triad.ToAttacker) + dmgTag,
		retortPrefix + string(triad.ToRoom)
}

// buildGenericCounterTauntMessages is the Task 10 retort narration, kept only
// as the fallback for environments where the counter-defy pool is not loaded.
func buildGenericCounterTauntMessages(countererName, counteredName string, damage, counteredMaxCP int) (countererMsg, taunterMsg, roomMsg string) {
	if damage > 0 {
		dmgDesc := GetConvictionDamageDescription(damage, counteredMaxCP)
		return fmt.Sprintf(retortPrefix+`You throw %s's words right back in their face! (<ansi fg="damage">%s</ansi>)`, counteredName, dmgDesc),
			fmt.Sprintf(retortPrefix+`%s throws your words right back in your face! (<ansi fg="damage">%s</ansi>)`, countererName, dmgDesc),
			fmt.Sprintf(retortPrefix+`%s throws %s's words right back!`, countererName, counteredName)
	}
	return fmt.Sprintf(retortPrefix+`You snap back at %s, but the words fail to bite!`, counteredName),
		fmt.Sprintf(retortPrefix+`%s snaps back at you, but the words fail to bite!`, countererName),
		fmt.Sprintf(retortPrefix+`%s snaps back at %s!`, countererName, counteredName)
}
