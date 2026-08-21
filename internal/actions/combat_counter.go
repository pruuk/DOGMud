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
// Narration is channel-correct (U6b Task 11), rendered by internal/combat
// from the counter-* pools in defense-messages/. SEQUENCING (the Task 10 wart,
// fixed by Task 11): counterSkillMoveExit does NOT dispatch — the counter
// would print before the move's own outcome, because messages render in call
// order and the wrappers narrate AFTER ExecuteX returns. Instead the
// CounterResult rides up on the action's result struct, and the command
// wrapper calls DispatchCounterMessages after its own outcome text — the same
// flow the defence triads use. The defy counter-taunt keeps dispatching from
// its exit (Task 10's review flagged only the skill-move ordering; the taunt
// path was accepted as-is), but its narration now comes from the counter-defy
// pool via combat.BuildCounterTauntMessages.

import (
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
// It resolves the counter-swing (damage lands HERE) but dispatches nothing:
// the result rides up on the action's result struct so the command wrapper
// can speak it AFTER the move's own outcome (DispatchCounterMessages) —
// messages render in call order, so dispatching from inside the action put
// the counter before the move it answered (the Task 10 wart).
//
// Recursion guard: a result produced under IsCounter is refused here — the
// tier never fires FROM a counter. ExecuteCounter marks its own swing
// IsCounter, closing the loop.
func counterSkillMoveExit(actor Actor, defender *characters.Character,
	move combat.SkillMoveResult, channel combat.AttackChannel, sameRoom bool) combat.CounterResult {

	if !move.Defence.DefensiveCrit || move.IsCounter {
		return combat.CounterResult{}
	}
	return combat.ExecuteCounter(defender, actor.GetCharacter(), channel, sameRoom)
}

// DispatchCounterMessages routes the channel-correct counter narration:
// private lines to whichever participants are players, one visual line to the
// room. Command wrappers call it AFTER rendering the move's own outcome so
// the counter reads as the answer it is (the Task 11 ordering fix). actor is
// the COUNTERED party (the one whose move was crit-defended); the counterer's
// line routes via res.CountererUserId. Safe to call unconditionally — a
// result that never countered dispatches nothing.
func DispatchCounterMessages(actor Actor, res combat.CounterResult) {
	if !res.Countered {
		return
	}
	exclude := []int{}
	if res.DefenderMsg != "" && res.CountererUserId > 0 {
		if u := users.GetByUserId(res.CountererUserId); u != nil {
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

	// Narration (U6b Task 11): the counter-defy pool — the jeer turned back.
	// No numbers, no interrupt framing — the taunt already resolved; the
	// counter is what the counterer does with the opening.
	countererMsg, taunterMsg, roomMsg := combat.BuildCounterTauntMessages(
		target.Char.Name, char.Name,
		res.Defence.AttackerCrit, res.Damage, maxOfOne(char.ConvictionMax.Value))

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
