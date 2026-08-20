package actions

// U6b Task 10 — the counter tier's actions-side wiring.
//
// Two entry points live here:
//
//   - counterSkillMoveExit: fires combat.ExecuteCounter at every
//     ExecuteSkillMove consumer's defensive-crit exit (the special moves and
//     ExecuteFire). It refuses results produced under IsCounter, so melee's
//     auto-trip/auto-bash (which ride the seam AS counters) can never chain.
//   - executeCounterTaunt: the defy carve-out. A defy crit COUNTER-TAUNTS
//     instead of counter-swinging, and the wiring lives HERE (not in
//     internal/combat) because taunt resolution needs this package and
//     internal/combat can never import it.
//
// Narration is the generic Task 10 text (Task 11 ships channel-correct
// counter triads). It is dispatched from these helpers because the counter is
// a tier side effect, not part of the action's own result contract; callers
// keep rendering their own outcome exactly as before.

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// counterSkillMoveExit fires the counter tier at one skill-move exit: the
// DEFENDER of the move earned a defensive crit and answers the ACTOR who
// attempted it. sameRoom carries the reach gate (false only for the
// cross-room shot, the one uncounterable attack).
//
// Recursion guard: a result produced under IsCounter is refused here — the
// tier never fires FROM a counter. ExecuteCounter marks its own swing
// IsCounter, closing the loop.
func counterSkillMoveExit(actor Actor, defender *characters.Character,
	move combat.SkillMoveResult, channel combat.AttackChannel, sameRoom bool) combat.CounterResult {

	if !move.Defence.DefensiveCrit || move.IsCounter {
		return combat.CounterResult{}
	}
	res := combat.ExecuteCounter(defender, actor.GetCharacter(), channel, sameRoom)
	if !res.Countered {
		return res
	}
	dispatchCounterSwingMessages(actor, defender, res)
	return res
}

// dispatchCounterSwingMessages routes the generic counter narration: private
// lines to whichever participants are players, one visual line to the room.
func dispatchCounterSwingMessages(actor Actor, defender *characters.Character, res combat.CounterResult) {
	exclude := []int{}
	if res.DefenderMsg != "" {
		if u := users.GetByUserId(defender.GetUserId()); u != nil {
			u.SendText(messaging.CategoryHitMelee, res.DefenderMsg)
			exclude = append(exclude, u.UserId)
		}
	}
	if res.AttackerMsg != "" {
		actor.SendText(messaging.CategoryHitMelee, res.AttackerMsg)
		if actor.GetUserId() > 0 {
			exclude = append(exclude, actor.GetUserId())
		}
	}
	if room := actor.GetRoom(); room != nil && res.RoomMsg != "" {
		room.SendTextVisual(messaging.CategoryHitMelee, res.RoomMsg, exclude...)
	}
}

// CounterTauntResult reports the defy carve-out's outcome for callers and
// tests. The counter-taunt is free for the counterer; Defence carries the
// original taunter's (charged, progressed) defy of it.
type CounterTauntResult struct {
	Fired   bool
	Fumbled bool
	Damage  int
	Defence combat.ChannelDefenceResult
}

// executeCounterTaunt is the defy carve-out's cost-free entry point: the
// counterer (who just defy-critted a taunt) throws a counter-taunt at the
// original taunter, reusing ONLY the contest + damage shape of taunt
// resolution. Deliberately bypassed, each an owner decision (2026-08-19):
//
//   - NO special-move cooldown: neither checked nor consumed. ExecuteTaunt's
//     CooldownReady/TryCooldown pair is for paid, chosen actions; a counter
//     is neither.
//   - NO U8 admission cost: admitFullCost is never called. The counter is
//     free for the counterer. (The TARGET still pays to defy it through the
//     seam — the countered-party economy.)
//   - NO aggro mutation: no SetAggro, no ForceTauntAggro, no RoundsWaiting.
//     A counter answers an engagement that already exists; it must not
//     re-point anyone's target.
//
// It also earns no progression and records no special-move analytics — those
// belong to chosen actions.
//
// Recursion: this entry point IS the counter (the taunt-channel equivalent of
// carrying IsCounter). It never inspects its own contest's DefensiveCrit, and
// the tier's only taunt wiring sits at ExecuteTaunt's non-counter call site,
// so a defy crit against the counter-taunt ends the chain.
//
// A fumbled counter-taunt just fizzles: no self-damage — the fumble
// self-damage in ExecuteTaunt is part of the paid action's risk, which this
// free answer does not carry.
func executeCounterTaunt(counterer, target *characters.Character) CounterTauntResult {
	result := CounterTauntResult{}
	if counterer == nil || target == nil {
		return result
	}
	if counterer.Health < 1 || target.Health < 1 {
		return result
	}
	// CounterDamagePercent 0 is the counter tier's master off-switch; the
	// counter-taunt honours it even though its damage is taunt-shaped (the
	// fixed 0.5 taunt base), not knob-priced.
	if float64(configs.GetBalanceConfig().CounterDamagePercent) <= 0 {
		return result
	}

	cfg := configs.GetBalanceConfig()

	// The counterer's half mirrors ExecuteTaunt's: Charisma + RAW rhetoric
	// rank (the seam applies SkillWeight), conviction-depletion on Mult.
	convMult := combat.ResourceMultiplier(counterer.Conviction,
		counterer.EffectivePoolMax(characters.PoolConviction),
		float64(cfg.ConvictionPenaltyMax))
	side := combat.AttackSide{
		Stat:      counterer.Stats.Charisma.ValueAdj,
		StatName:  "charisma",
		Skill:     skills.Rhetoric,
		SkillRank: counterer.GetSkillLevel(skills.Rhetoric),
		Mult:      convMult,
	}

	// ONE contest through the seam: the original taunter defies the
	// counter-taunt, and that defence is charged and progressed exactly like
	// any other (the countered-party economy).
	out := combat.ResolveChannelAttack(combat.ChannelSocial, side, counterer, target)
	result.Fired = true
	result.Defence = out

	// A fumbled counter-taunt fizzles, with none of the paid action's
	// self-damage.
	if out.AttackerFumble {
		result.Fumbled = true
		return result
	}

	// Taunt's damage shape, verbatim: raw conviction damage at the fixed 0.5
	// taunt base, depletion-scaled, crit-or-mitigated on the seam's verdict,
	// then scaled by the same contest's defence multiplier.
	rawDmg := combat.CalcRawDamage(
		counterer.Stats.Charisma.ValueAdj,
		side.SkillRank,
		0.5, // taunt base item multiplier
		combat.ChannelConviction,
	)
	rawDmg *= convMult
	dmg := combat.CritOrMitigatedDamage(
		rawDmg,
		side.SkillRank,
		out.AttackerCrit,
		target.GetConvictionMitigation(),
		combat.MitigationCap(combat.ChannelConviction),
	)
	if mult := out.DamageMultiplier; mult < 1.0 {
		dmg = int(math.Round(float64(dmg) * mult))
		if dmg < 1 && mult > 0 {
			dmg = 1
		}
	}
	if dmg > 0 {
		target.ApplyHarm(characters.PoolConviction, dmg,
			state.ActorRef{UserId: counterer.GetUserId(), MobInstanceId: counterer.MobInstanceId})
	}
	result.Damage = dmg
	return result
}

// counterTauntExit wires the defy carve-out at ExecuteTaunt's defensive-crit
// exit and dispatches the generic narration. actor is the ORIGINAL taunter
// (now being counter-taunted); target identifies the counterer.
func counterTauntExit(actor Actor, char *characters.Character, target AggroTarget,
	out combat.ChannelDefenceResult) CounterTauntResult {

	if !out.DefensiveCrit || target.Char == nil {
		return CounterTauntResult{}
	}
	res := executeCounterTaunt(target.Char, char)
	if !res.Fired {
		return res
	}

	// Narration: no numbers, no interrupt framing — the taunt already
	// resolved; the counter is what the counterer does with the opening.
	counterName := target.Char.Name
	taunterName := char.Name
	var countererMsg, taunterMsg, roomMsg string
	if res.Damage > 0 {
		dmgDesc := combat.GetConvictionDamageDescription(res.Damage, maxOfOne(char.ConvictionMax.Value))
		countererMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ RETORT!</ansi> You throw %s's taunt right back in their face! (<ansi fg="damage">%s</ansi>)`,
			taunterName, dmgDesc)
		taunterMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ RETORT!</ansi> %s throws your taunt right back in your face! (<ansi fg="damage">%s</ansi>)`,
			counterName, dmgDesc)
		roomMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ RETORT!</ansi> %s throws %s's taunt right back!`,
			counterName, taunterName)
	} else {
		countererMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ RETORT!</ansi> You snap back at %s, but the words fail to bite!`,
			taunterName)
		taunterMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ RETORT!</ansi> %s snaps back at you, but the words fail to bite!`,
			counterName)
		roomMsg = fmt.Sprintf(
			`<ansi fg="cyan-bold">⚔ RETORT!</ansi> %s snaps back at %s!`,
			counterName, taunterName)
	}

	exclude := []int{}
	if target.UserId > 0 {
		if u := users.GetByUserId(target.UserId); u != nil {
			u.SendText(messaging.CategoryTauntSuccess, countererMsg)
			exclude = append(exclude, target.UserId)
		}
	}
	actor.SendText(messaging.CategoryTauntSuccess, taunterMsg)
	if actor.GetUserId() > 0 {
		exclude = append(exclude, actor.GetUserId())
	}
	if room := rooms.LoadRoom(char.RoomId); room != nil {
		room.SendTextVisual(messaging.CategoryTauntSuccess, roomMsg, exclude...)
	}
	return res
}

// maxOfOne guards a max-pool denominator for damage descriptions.
func maxOfOne(v int) int {
	if v <= 0 {
		return 1
	}
	return v
}
