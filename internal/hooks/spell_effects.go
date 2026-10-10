package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Spell effect unification (parity slices 3a and 3b,
// docs/superpowers/specs/completed/2026-09-28-spell-effect-unification-design.md).
//
// Every spell effect on one target is applied through one spellEffectCtx and
// one dispatcher, whoever casts it and whoever it hits. The four contested
// resolvers in spell_resolution.go keep their names and their one
// runSpellChannelAttack call each; after the contest they build a context and
// hand it here.

// recordSpell is the analytics seam for one resolved cast. It defaults to
// combat.RecordSpell; same-package tests replace it with a recorder and
// restore it with t.Cleanup, the pattern runSpellChannelAttack uses.
var recordSpell = combat.RecordSpell

// spellEffectCtx is everything one spell effect needs about one target.
type spellEffectCtx struct {
	casterChar *characters.Character // nil only in tests that pass no caster
	caster     actions.Actor         // *actions.UserActor, *actions.MobActor, or nil
	target     actions.Actor         // *actions.UserActor or *actions.MobActor, never nil
	room       *rooms.Room
	spell      *spells.SpellData
	magnitude  int
	out        combat.ChannelDefenceResult // zero-valued win for uncontested casts
}

func newSpellEffectCtx(casterChar *characters.Character, caster, target actions.Actor, room *rooms.Room,
	spell *spells.SpellData, magnitude int, out combat.ChannelDefenceResult) spellEffectCtx {
	return spellEffectCtx{casterChar: casterChar, caster: caster, target: target, room: room,
		spell: spell, magnitude: magnitude, out: out}
}

func actorUser(a actions.Actor) *users.UserRecord {
	if ua, ok := a.(*actions.UserActor); ok && ua != nil {
		return ua.User
	}
	return nil
}

func actorMob(a actions.Actor) *mobs.Mob {
	if ma, ok := a.(*actions.MobActor); ok && ma != nil {
		return ma.Mob
	}
	return nil
}

func actorRefOf(a actions.Actor) state.ActorRef {
	if a == nil {
		return state.ActorRef{}
	}
	return state.ActorRef{UserId: a.GetUserId(), MobInstanceId: a.GetMobInstanceId()}
}

func (c spellEffectCtx) casterUser() *users.UserRecord { return actorUser(c.caster) }
func (c spellEffectCtx) casterMob() *mobs.Mob          { return actorMob(c.caster) }
func (c spellEffectCtx) targetUser() *users.UserRecord { return actorUser(c.target) }
func (c spellEffectCtx) targetMob() *mobs.Mob          { return actorMob(c.target) }

func (c spellEffectCtx) targetChar() *characters.Character { return c.target.GetCharacter() }

// casterRef names the caster for aggro and harm attribution. It reads the
// actor, not the character: a mob's Character.MobInstanceId is not reliably
// set, its Mob.InstanceId is.
func (c spellEffectCtx) casterRef() state.ActorRef {
	if c.caster == nil {
		return charActorRef(c.casterChar)
	}
	return actorRefOf(c.caster)
}

func (c spellEffectCtx) targetRef() state.ActorRef { return actorRefOf(c.target) }

// viewerId is the user id mob names are rendered for: the player caster's,
// or 0 when a mob casts.
func (c spellEffectCtx) viewerId() int {
	if u := c.casterUser(); u != nil {
		return u.UserId
	}
	return 0
}

// spellEffectName is a party's name exactly as the spell lines print it, and
// so exactly as SendTrio must hide it: players in the username tag, mobs
// through mobDisplayName with the room's duplicate index.
func spellEffectName(a actions.Actor, room *rooms.Room, viewerId int) string {
	switch v := a.(type) {
	case *actions.UserActor:
		return fmt.Sprintf(`<ansi fg="username">%s</ansi>`, v.User.Character.Name)
	case *actions.MobActor:
		return mobDisplayName(v.Mob, room, viewerId)
	}
	return "something"
}

func (c spellEffectCtx) casterName() string { return spellEffectName(c.caster, c.room, c.viewerId()) }
func (c spellEffectCtx) targetName() string { return spellEffectName(c.target, c.room, c.viewerId()) }

// hiddenFromRoom reports which parties are hidden mobs, unseen by every
// observer of a room line (mobDisplayName's room-wide rule, #382). Read it
// when the line's names are read: a harmful spell's commit can reveal the
// target between the two, and the line then still prints the name it read.
func (c spellEffectCtx) hiddenFromRoom() (caster, target bool) {
	return actorHiddenFromRoom(c.caster), actorHiddenFromRoom(c.target)
}

func actorHiddenFromRoom(a actions.Actor) bool {
	m := actorMob(a)
	return m != nil && mobHiddenFrom(m, 0)
}

// roomName is a party's name in a spell's room line: printed, the name the
// caster's own line prints, unless the party was hidden (hiddenFromRoom). A
// player caster who sees the hidden reads a hidden mob's name in its own
// line, and the room line reused that string, naming the mob to the room.
func roomName(hidden bool, printed string) string {
	if hidden {
		return messaging.UnseenFigure(messaging.SightNone)
	}
	return printed
}

func (c spellEffectCtx) critTag() string {
	if c.out.AttackerCrit {
		return critMarker
	}
	return ""
}

func (c spellEffectCtx) category() messaging.Category { return spellSchoolCategory(c.spell) }

// audience is the SendTrio audience for a line between caster and target.
// A mob side gets no private line (spellAudience stores no nil recipient),
// and the room line excludes whichever sides are players.
func (c spellEffectCtx) audience() messaging.Audience {
	return spellAudience(c.casterUser(), c.casterName(), c.targetUser(), c.targetName(), c.room)
}

func spellSourceTarget(a actions.Actor) combat.SourceTarget {
	if a != nil && a.IsPlayer() {
		return combat.User
	}
	return combat.Mob
}

// applySpellEffect applies one spell effect to one target, whoever casts it
// and whoever it hits, and returns the damage it dealt (0 for effects that
// deal none). Each effect has one applier (spell_effects.go for the harmful
// ones, spell_help_effects.go for the rest). Charm binds only a mob, and
// applyMobEffect_charm refuses a caster that is not a player; charm on
// anything else, and any effect with no applier, is the default arm.
func applySpellEffect(c spellEffectCtx) int {
	switch c.spell.EffectType {
	case "damage":
		return applySpellDamage(c)
	case "dot":
		return applySpellDot(c)
	case "knockdown":
		return applySpellKnockdown(c)
	case "condition":
		return applySpellConditionEffect(c)
	case "heal":
		return applySpellHeal(c)
	case "shield":
		return applySpellShield(c)
	case "purge":
		return applySpellPurge(c)
	case "charm":
		if m := c.targetMob(); m != nil {
			return applyMobEffect_charm(c.casterUser(), m, c.room, c.spell, c.out, c.targetName())
		}
	}
	return applySpellDefaultEffect(c)
}

// commitHarmfulSpellAggro is the one place a harmful spell starts a fight,
// for every pairing. fresh is whether the target was out of combat BEFORE
// this cast landed (the applier reads it first, because harm can end the
// target's fight). The target turns on its caster only when fresh, so an
// established fight is not yanked around; the caster turns on the target
// when it is not already fighting.
//
// A player's harm on a mob is also an assault (owner ruling, 2026-09-28):
// actions.SeedAggression fires PlayerAttackedMob on every cast and, when
// fresh, the opinion bump and the assault crime. Freshness is judged per
// target from the mob's own prior combat, exactly as usercommands/throw.go's
// engageAfterThrow judges it for an area throw.
func commitHarmfulSpellAggro(c spellEffectCtx, fresh bool) {
	tc := c.targetChar()
	if fresh {
		targeting.Commit(tc, c.casterRef(), targeting.ReasonAttack)
	}
	if c.casterChar != nil && !c.casterChar.IsInCombat() {
		targeting.Commit(c.casterChar, c.targetRef(), targeting.ReasonAttack)
	}
	if u, m := c.casterUser(), c.targetMob(); u != nil && m != nil {
		actions.SeedAggression(u, m, c.room, fresh)
	}
}

// creditSpellDamage remembers who hurt a mob, as melee does: a player caster
// is credited with the damage, and a mob caster charmed by a player credits
// that player (combat.go AttackPlayerVsMob and AttackMobVsMob). Death
// processing reads PlayerDamage for the murder upgrade, faction rep, bounty,
// quest kill credit and item procs, so a spell kill without it counts for
// no one.
//
// It runs BEFORE the harm. ApplyHarm only queues the death today, so melee's
// after-the-harm order is safe too, but crediting first keeps the kill
// attributed even if a death is ever resolved inline.
func creditSpellDamage(c spellEffectCtx, dmg int) {
	m := c.targetMob()
	if m == nil || dmg <= 0 {
		return
	}
	if u := c.casterUser(); u != nil {
		m.Character.TrackPlayerDamage(u.UserId, dmg)
		return
	}
	if cm := c.casterMob(); cm != nil {
		if charmedUserId := cm.Character.GetCharmedUserId(); charmedUserId > 0 {
			m.Character.TrackPlayerDamage(charmedUserId, dmg)
		}
	}
}

// applySpellDamage is the one damage applier (slice 3a). The resolver ran
// the ONE contest; this consumes it. A defended cast lands partial damage, a
// defensive crit negates it, and either way the cast was an attack.
func applySpellDamage(c spellEffectCtx) int {
	tc := c.targetChar()
	fresh := !tc.IsInCombat()
	dmg := scaleSpellDamageByDefence(
		calcSpellDamageForCharacter(c.spell, c.casterChar, tc, c.magnitude, c.out.AttackerCrit), c.out)
	sendSpellChannelDefenceMessages(c.room, c.category(), c.out,
		spellDefenceIdentity(c.casterChar, c.casterUser(), c.room),
		spellDefenceIdentity(tc, c.targetUser(), c.room), c.spell.Name, c.casterUser(), c.targetUser())
	if c.out.DefensiveCrit {
		dmg = 0
	} else {
		creditSpellDamage(c, dmg)
		tc.ApplyHarm(characters.PoolHealth, dmg, c.casterRef())
		cancelDamageConditions(tc)
		// on_spell_hit item procs fire only on a harm hit that dealt damage;
		// the proc's own chance and cooldown pace an area cast.
		if dmg > 0 {
			fireItemProc(behaviortree.EventContext{EventType: "on_spell_hit"}, c.casterChar, tc, nil, dmg)
		}
	}
	commitHarmfulSpellAggro(c, fresh)
	if c.out.Defended {
		return dmg // the defence triad above already told everyone
	}
	dmgDesc := combat.GetDamageDescription(dmg, tc.HealthMax.Value)
	casterHidden, targetHidden := c.hiddenFromRoom()
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(c.category(), fmt.Sprintf(
			`Your %s strikes %s! (<ansi fg="damage">%s</ansi>)%s`,
			c.spell.Name, c.targetName(), dmgDesc, c.critTag())),
		Actee: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> strikes you! (<ansi fg="damage">%s</ansi>)%s`,
			c.casterName(), c.spell.Name, dmgDesc, c.critTag())),
		Observer: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> strikes %s!`,
			roomName(casterHidden, c.casterName()), c.spell.Name, roomName(targetHidden, c.targetName()))),
	}, c.audience())
	return dmg
}

// spellCasterStatAndSkill is the caster side of a spell's duration: the
// spell's own primarystat through CasterStatValue and the school's cast
// skill (Manifestation for that school, Spellcasting otherwise), the formula
// the mob-on-player dot always used. A nil caster reads stat 100 and skill 0,
// the old anonymous-caster default, which only tests reach.
//
// Not actions.GetSpellStatAndSkill: that one returns Perception, the FOLD
// stat, and would move every dot off its declared primarystat.
func spellCasterStatAndSkill(spell *spells.SpellData, caster *characters.Character) (stat int, skill int) {
	if caster == nil {
		return 100, 0
	}
	castSkill := skills.Spellcasting
	if spell.HasSchool(spells.SchoolManifestation) {
		castSkill = skills.Manifestation
	}
	return spell.CasterStatValue(caster.Stats), caster.GetSkillLevel(castSkill)
}

// applySpellDot is the one damage-over-time applier (slice 3a). The
// affliction is binary: it lands only on an attack win, and a defended cast
// narrates the defence triad and applies nothing. Either way the cast was an
// attack.
//
// Unlike damage and knockdown, the dot does NOT credit its caster in the
// mob's PlayerDamage (creditSpellDamage): the ticks harm with an anonymous
// source, so a dot kill still counts for no one. Crediting it needs the
// caster carried on the condition record; that is a filed follow-up.
func applySpellDot(c spellEffectCtx) int {
	tc := c.targetChar()
	fresh := !tc.IsInCombat()
	if c.out.Defended {
		sendSpellChannelDefenceMessages(c.room, c.category(), c.out,
			spellDefenceIdentity(c.casterChar, c.casterUser(), c.room),
			spellDefenceIdentity(tc, c.targetUser(), c.room), c.spell.Name, c.casterUser(), c.targetUser())
		commitHarmfulSpellAggro(c, fresh)
		return 0
	}
	stat, skill := spellCasterStatAndSkill(c.spell, c.casterChar)
	dotDuration := calcSpellDuration(c.spell.BaseFolds, skill, stat) / 3
	if dotDuration < 3 {
		dotDuration = 3
	}
	// Condition 121 ticks every round, so dotDuration is the trigger count.
	// The record's negative magnitude is the harm per tick, floored at one.
	dotAmount := c.magnitude
	if dotAmount < 1 {
		dotAmount = 1
	}
	// Names are read BEFORE the condition lands: AddConditionMagnitude below
	// can add the "poisoned" adjective to the target's own rendered name, and
	// SendTrio's redaction must see the exact string the line prints.
	casterName, targetName := c.casterName(), c.targetName()
	casterHidden, targetHidden := c.hiddenFromRoom()
	// The character door, on purpose: an immune target refuses the record,
	// and nothing that did not happen may be narrated.
	afflicted := tc.AddConditionMagnitude(conditions.ConditionIdPoisoned, dotDuration, -float64(dotAmount), "spell") == nil
	commitHarmfulSpellAggro(c, fresh)
	if !afflicted {
		return 0
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(c.category(), fmt.Sprintf(
			`Your %s afflicts %s!%s`, c.spell.Name, targetName, c.critTag())),
		Actee: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> afflicts you!%s`, casterName, c.spell.Name, c.critTag())),
		Observer: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> afflicts %s!`,
			roomName(casterHidden, casterName), c.spell.Name, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// applySpellKnockdown is the one knockdown applier (slice 3a). The defence
// scales the DAMAGE (a defended cast still lands a partial hit, a defensive
// crit negates it), while the knockdown is binary and lands only on an
// attack win: ExecuteSkillMove's Hit/StatusApplied split.
func applySpellKnockdown(c spellEffectCtx) int {
	tc := c.targetChar()
	fresh := !tc.IsInCombat()
	dmg := scaleSpellDamageByDefence(
		calcSpellDamageForCharacter(c.spell, c.casterChar, tc, c.magnitude, c.out.AttackerCrit), c.out)
	// Names are read before ApplyHarm and TransitionToSupine: a lethal hit
	// adds the "dead" adjective to the target's own rendered name, and
	// SendTrio's redaction must see the exact string the line prints.
	casterName, targetName := c.casterName(), c.targetName()
	casterHidden, targetHidden := c.hiddenFromRoom()
	if c.out.DefensiveCrit {
		dmg = 0
	} else {
		creditSpellDamage(c, dmg)
		tc.ApplyHarm(characters.PoolHealth, dmg, c.casterRef())
		cancelDamageConditions(tc)
		if dmg > 0 {
			fireItemProc(behaviortree.EventContext{EventType: "on_spell_hit"}, c.casterChar, tc, nil, dmg)
		}
	}
	// Spell knockdowns put the target on its back (Supine); a target already
	// grappled or down takes the damage but no knockdown is narrated.
	knocked := false
	if !c.out.Defended {
		knocked = true
		if err := tc.Position.TransitionToSupine(
			position.SupineData{MinRecoveryRounds: 1},
			state.TransitionReason{Trigger: position.TriggerKnockdownSpell},
		); err != nil {
			mudlog.Warn("applySpellKnockdown: TransitionToSupine failed", "target", c.targetRef(), "err", err)
			knocked = false
		}
	}
	commitHarmfulSpellAggro(c, fresh)
	sendSpellChannelDefenceMessages(c.room, c.category(), c.out,
		spellDefenceIdentity(c.casterChar, c.casterUser(), c.room),
		spellDefenceIdentity(tc, c.targetUser(), c.room), c.spell.Name, c.casterUser(), c.targetUser())
	if c.out.Defended {
		return dmg // the triad above already narrated it; a defended cast never knocks down
	}
	dmgDesc := combat.GetDamageDescription(dmg, tc.HealthMax.Value)
	if knocked {
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(c.category(), fmt.Sprintf(
				`Your %s slams %s to the ground! (<ansi fg="damage">%s</ansi>)%s`,
				c.spell.Name, targetName, dmgDesc, c.critTag())),
			Actee: messaging.Say(c.category(), fmt.Sprintf(
				`%s's <ansi fg="cyan">%s</ansi> slams you to the ground! (<ansi fg="damage">%s</ansi>)%s`,
				casterName, c.spell.Name, dmgDesc, c.critTag())),
			Observer: messaging.Say(c.category(), fmt.Sprintf(
				`%s's <ansi fg="cyan">%s</ansi> knocks %s to the ground!`,
				roomName(casterHidden, casterName), c.spell.Name, roomName(targetHidden, targetName))),
		}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
		return dmg
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(c.category(), fmt.Sprintf(
			`Your %s strikes %s, but %s is already down. (<ansi fg="damage">%s</ansi>)%s`,
			c.spell.Name, targetName, targetName, dmgDesc, c.critTag())),
		Actee: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> strikes you, but you're already down. (<ansi fg="damage">%s</ansi>)%s`,
			casterName, c.spell.Name, dmgDesc, c.critTag())),
		Observer: messaging.NoLine,
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return dmg
}

// recordSpellResolution records one resolved (not backfired) cast for every
// pairing. A defended cast records in the old fizzle column but keeps its
// partial damage (Stage 30.1).
func recordSpellResolution(c spellEffectCtx, dmg int) {
	recordSpell(spellSourceTarget(c.caster), spellSourceTarget(c.target),
		!c.out.Defended, c.out.AttackerCrit, false, c.out.Defended, dmg,
		c.out.AttackRollZScore, c.casterChar, c.targetChar(), util.GetRoundCount())
}

// uncontestedSpellResult is the contest result a help spell (attack_type
// none) resolves with: an attack win at full strength and no crit. A help
// spell never enters the contest, the only source of a crit, so help spells
// do not crit (owner ruling 3, 2026-09-28).
func uncontestedSpellResult() combat.ChannelDefenceResult {
	return combat.ChannelDefenceResult{DamageMultiplier: 1}
}

// resolveHelpSpell is the one uncontested step every resolver takes for a
// help spell (attack_type none), whoever casts it and whoever it lands on:
// no contest, so no fumble, backfire, interrupt or counter; the effect
// applies and the cast is recorded as landed. It always reports landed:
// there was no defence to beat. A mob's help spell on a player used to be
// contested and then apply nothing (audit row 3).
func resolveHelpSpell(c spellEffectCtx) bool {
	recordSpellResolution(c, applySpellEffect(c))
	return true
}

// applySpellBackfire resolves a fumbled cast for every caster kind: the
// caster takes a quarter of the magnitude (at least one), is told if it is a
// player, the room sees it, and it is recorded.
func applySpellBackfire(c spellEffectCtx) {
	backfireDmg := c.magnitude / 4
	if backfireDmg < 1 {
		backfireDmg = 1
	}
	if c.casterChar != nil {
		c.casterChar.ApplyHarm(characters.PoolHealth, backfireDmg, c.casterRef())
	}
	name := c.casterName()
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(messaging.CategorySpellDisruption,
			`<ansi fg="red">Your spell backfires violently, wounding you!</ansi>`),
		Actee: messaging.NoLine,
		Observer: messaging.Say(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="red">%s's spell backfires!</ansi>`, name)),
	}, spellAudience(c.casterUser(), name, nil, messaging.NoName, c.room))
	recordSpell(spellSourceTarget(c.caster), spellSourceTarget(c.target), false, false, true, false, 0,
		c.out.AttackRollZScore, c.casterChar, c.targetChar(), util.GetRoundCount())
}

// maybeInterruptSpellOnTarget cancels any character's in-progress cast when
// spellId is a configured boss-interrupt disruption spell
// (Balance.BossInterruptSpellIds) and the character is casting. It reuses
// actions.InterruptTargetCast (conviction refund, cast cancel, and the
// CastInterrupted event for a player). Returns whether a cast was cancelled.
func maybeInterruptSpellOnTarget(target *characters.Character, spellId string, by state.ActorRef) bool {
	if target == nil {
		return false
	}
	if !configs.GetBalanceConfig().IsBossInterruptSpell(spellId) {
		return false
	}
	if !target.IsCasting() {
		return false
	}
	return actions.InterruptTargetCast(target, by)
}

// interruptSpellTarget runs the boss-interrupt for every pairing, after the
// backfire check (a botched cast cannot interrupt) and whether or not the
// target defends the damage: the interrupt is the point of the spell.
func interruptSpellTarget(c spellEffectCtx) {
	if !maybeInterruptSpellOnTarget(c.targetChar(), c.spell.SpellId, c.casterRef()) {
		return
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="cyan-bold">Your %s scrambles %s's focus. The spell collapses!</ansi>`,
			c.spell.Name, c.targetName())),
		Actee: messaging.Say(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="cyan-bold">%s's %s scrambles your focus. Your spell collapses!</ansi>`,
			c.casterName(), c.spell.Name)),
		Observer: messaging.Say(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="cyan">%s's spell collapses!</ansi>`, roomName(actorHiddenFromRoom(c.target), c.targetName()))),
		// A disruption is heard as well as seen (#242, owner ruling R4).
		ObserverSound: messaging.Say(messaging.CategorySpellDisruption, messaging.SoundChantBreaksOff),
	}, c.audience())
}
