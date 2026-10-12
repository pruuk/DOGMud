package actions

// U6b Task 10 — the counter tier's actions-side wiring.
//
// Three entry points live here:
//
//   - counterSkillMoveExit: fires combat.ExecuteCounter at every
//     single-target ExecuteSkillMove consumer's defensive-crit exit (the
//     special moves and ExecuteFire; the area drain has none, because an
//     area attack earns no counter). It refuses results produced under
//     IsCounter, so melee's auto-trip/auto-bash (which ride the seam AS
//     counters) can never chain.
//   - FireCounterTaunt: exported. The one defy dispatch, shared by taunt's
//     counterTauntExit below and the spell exit in internal/hooks, for a
//     defied charm. A defy crit COUNTER-TAUNTS instead of counter-swinging,
//     and the wiring lives HERE (not in internal/combat) because taunt
//     resolution needs this package and internal/combat can never import
//     it.
//   - executeCounterTaunt (above FireCounterTaunt): the defy carve-out's
//     cost-free contest and damage primitive, called by FireCounterTaunt.
//
// Narration is defence-correct (U6b Task 11), rendered by internal/combat
// from the counter-* pools in defense-messages/. SEQUENCING (the Task 10 wart,
// fixed by Task 11): counterSkillMoveExit does NOT dispatch — the counter
// would print before the move's own outcome, because messages render in call
// order and the wrappers narrate AFTER ExecuteX returns. Instead the
// CounterResult rides up on the action's result struct, and the command
// wrapper calls DispatchCounterMessages after its own outcome text — the same
// flow the defence triads use. The defy counter-taunt dispatches straight
// from FireCounterTaunt (Task 10's review flagged only the skill-move
// ordering; the taunt path was accepted as-is at the time), and its
// narration comes from the counter-defy pool via
// combat.BuildCounterTauntMessages.

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
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
// cross-room shot, the one single-target attack that cannot be countered).
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
	move combat.SkillMoveResult, shape combatvocab.Attack, sameRoom bool) combat.CounterResult {

	if !move.Defence.DefensiveCrit || move.IsCounter {
		return combat.CounterResult{}
	}
	return combat.ExecuteCounter(defender, actor.GetCharacter(), shape, move.Defence.Defence, sameRoom)
}

// DispatchCounterMessages routes the defence-correct counter narration:
// private lines to whichever participants are players, one visual line to the
// room. Command wrappers call it AFTER rendering the move's own outcome so
// the counter reads as the answer it is (the Task 11 ordering fix). actor is
// the COUNTERED party (the one whose move was crit-defended); the counterer's
// line routes via res.CountererUserId. Safe to call unconditionally — a
// result that never countered dispatches nothing.
func DispatchCounterMessages(actor Actor, res combat.CounterResult) {
	var countered messaging.Recipient
	if actor.IsPlayer() {
		countered = actor
	}
	SendCounterTrio(actor.GetRoom(), res, countered, actor.GetUserId())
}

// SendCounterTrio delivers one counter's narration through messaging.SendTrio:
// the counterer's line, the countered party's line and the room's. A reader who
// cannot see the other party reads "something" in place of their name, and the
// room line reaches only observers who can see. The counterer is looked up
// from res.CountererUserId; countered is nil for a mob. Both the skill-move
// exits (DispatchCounterMessages) and the spell exits
// (hooks.fireSpellCounterTier) come through here.
func SendCounterTrio(room *rooms.Room, res combat.CounterResult, countered messaging.Recipient, counteredUserId int) {
	if !res.Countered {
		return
	}
	var counterer messaging.Recipient
	if res.CountererUserId > 0 {
		if u := users.GetByUserId(res.CountererUserId); u != nil {
			counterer = u
		}
	}
	aud := messaging.Audience{
		Actor:     counterer,
		ActorId:   res.CountererUserId,
		ActorName: res.CountererName,
		Actee:     countered,
		ActeeId:   counteredUserId,
		ActeeName: res.CounteredName,
	}
	if room != nil {
		aud.Room = room
	}
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategoryHitMelee, res.DefenderMsg),
		Actee:    messaging.Say(messaging.CategoryHitMelee, res.AttackerMsg),
		Observer: messaging.Say(messaging.CategoryHitMelee, res.RoomMsg),
	}, aud)
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
	//
	// Lighting plan 5b: this function holds only the characters, so the room
	// is loaded once here. Rhetoric is unity on the sight rows.
	room := combat.SightRoom(rooms.LoadRoom(counterer.RoomId))
	out := combat.ResolveChannelAttack(room, combatvocab.Rhetoric(combatvocab.TargetSingle), side, counterer, target)
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
		combat.MitigationCurveFor(combat.ChannelConviction),
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

// FireCounterTaunt is the defy answer, shared by taunt's exit here and the
// spell exit in internal/hooks (a defied charm). shape is the original
// attack (taunt or charm); counterer is the one whose defy critted; countered
// the one whose words were defied. A nil recipient reads no private line (a
// mob, or a player the caller could not resolve). The narration is the
// counter-defy pool via combat.BuildCounterTauntMessages; the room line goes
// to everyone who can see, the two private lines to whichever party is a
// player. The dispatch parameters are messaging.Recipient rather than a
// concrete *users.UserRecord (the taunt exit's caller is an Actor, which can
// wrap a UserRecord that never sits in the users registry (a test double),
// so a registry lookup silently drops the line; Actor already satisfies
// Recipient, the same seam DispatchCounterMessages/SendCounterTrio use for
// this exact problem, and SendText delivers correctly either way). Dispatch
// goes through messaging.SendTrio, the same seam SendCounterTrio uses, so a
// reader who cannot see the other party reads no name and the room line
// reaches only observers who can see.
func FireCounterTaunt(room *rooms.Room, shape combatvocab.Attack, counterer, countered *characters.Character,
	countererRecipient messaging.Recipient, countererId int,
	counteredRecipient messaging.Recipient, counteredId int) CounterTauntResult {

	// A counter answers one deliberate attack at one target (owner ruling):
	// the same gate the swing primitive carries, here because a defy win
	// never reaches it.
	if shape.Targeting != combatvocab.TargetSingle {
		return CounterTauntResult{}
	}

	// A silenced counterer cannot answer at all (M6 slice 3), exactly as a
	// silenced taunter cannot taunt. Refused before the contest, so the
	// countered party defends nothing and no conviction moves.
	if counterer != nil && counterer.SpeechState() == characters.Silenced {
		return CounterTauntResult{}
	}

	res := executeCounterTaunt(counterer, countered)
	if !res.Fired {
		return res
	}

	countererMsg, counteredMsg, roomMsg := combat.BuildCounterTauntMessages(
		counterer, countered,
		res.Defence.AttackerCrit, res.Damage, maxOfOne(countered.ConvictionMax.Value))

	aud := messaging.Audience{
		Actor:     countererRecipient,
		ActorId:   countererId,
		ActorName: counterer.Name,
		Actee:     counteredRecipient,
		ActeeId:   counteredId,
		ActeeName: countered.Name,
	}
	if room != nil {
		aud.Room = room
	}
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategoryTauntSuccess, countererMsg),
		Actee:    messaging.Say(messaging.CategoryTauntSuccess, counteredMsg),
		Observer: messaging.Say(messaging.CategoryTauntSuccess, roomMsg),
	}, aud)
	return res
}

// counterTauntExit wires the defy carve-out at ExecuteTaunt's defensive-crit
// exit. actor is the ORIGINAL taunter (now being counter-taunted); target
// identifies the counterer. The counterer's recipient resolves through the
// users registry (the only way to reach it from an AggroTarget); the
// countered party dispatches through actor itself (see FireCounterTaunt),
// exactly as DispatchCounterMessages does for the swing-counter tier.
func counterTauntExit(actor Actor, char *characters.Character, target AggroTarget,
	out combat.ChannelDefenceResult) CounterTauntResult {

	if !out.DefensiveCrit || target.Char == nil {
		return CounterTauntResult{}
	}
	var countererRecipient messaging.Recipient
	if target.UserId > 0 {
		if u := users.GetByUserId(target.UserId); u != nil {
			countererRecipient = u
		}
	}
	var counteredRecipient messaging.Recipient
	if actor.IsPlayer() {
		counteredRecipient = actor
	}
	return FireCounterTaunt(rooms.LoadRoom(char.RoomId), combatvocab.Rhetoric(combatvocab.TargetSingle),
		target.Char, char, countererRecipient, target.UserId, counteredRecipient, actor.GetUserId())
}

// maxOfOne guards a max-pool denominator for damage descriptions.
func maxOfOne(v int) int {
	if v <= 0 {
		return 1
	}
	return v
}
