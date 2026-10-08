package hooks

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Spell effect unification, parity slice 3b
// (docs/superpowers/specs/completed/2026-09-28-spell-effect-unification-design.md):
// the helpful effects (condition, heal, shield, purge), the default arm and
// the one area-help target filler. Each applier serves every pairing (PM,
// PP, MS, MM, MP) through the spellEffectCtx spell_effects.go defines.

// selfCast reports whether the caster is its own target: a player's
// self-cast, the caster's own place in an area spell, or a mob's MS path.
func (c spellEffectCtx) selfCast() bool {
	return c.caster != nil && c.casterRef() == c.targetRef()
}

// selfCastAudience is the audience for a line about a caster acting on
// itself: the caster's private line (a player caster only) and the room
// line, which excludes a player caster. name is the caster's name exactly as
// the room line prints it.
func (c spellEffectCtx) selfCastAudience(name string) messaging.Audience {
	return spellAudience(c.casterUser(), name, nil, messaging.NoName, c.room)
}

// spellConditionTargetOf is the target as applySpellCondition's event door
// takes it: the player record or the mob, both of which queue the narrating
// events.Condition. Nil for an actor that is neither.
func spellConditionTargetOf(a actions.Actor) spellConditionTarget {
	if u := actorUser(a); u != nil {
		return u
	}
	if m := actorMob(a); m != nil {
		return m
	}
	return nil
}

// spellStatusDefended narrates a defended cast of a binary status effect
// (condition, heal, shield, purge, default) and reports whether it was
// defended. A defended status applies nothing: ExecuteSkillMove's
// StatusApplied split. Help spells are uncontested, so only a contested
// cast (a harmful condition, say) is ever defended.
func spellStatusDefended(c spellEffectCtx) bool {
	if !c.out.Defended {
		return false
	}
	sendSpellChannelDefenceMessages(c.room, c.category(), c.out,
		spellDefenceIdentity(c.casterChar, c.casterUser(), c.room),
		spellDefenceIdentity(c.targetChar(), c.targetUser(), c.room), c.spell.Name, c.casterUser(), c.targetUser())
	return true
}

// applySpellConditionEffect is the one condition applier (slice 3b): every
// condition the spell names goes through applySpellCondition's event door,
// which scales a light or sight by the caster and a heal- or damage-over-time
// by spellTickScale. A harmful condition starts the fight through
// commitHarmfulSpellAggro whether or not the target defended; for a player
// caster on a mob that is also the assault crime (owner ruling 2).
func applySpellConditionEffect(c spellEffectCtx) int {
	fresh := !c.targetChar().IsInCombat()
	if spellStatusDefended(c) {
		if c.spell.IsHarm() {
			commitHarmfulSpellAggro(c, fresh)
		}
		return 0
	}
	// Names are read BEFORE the condition lands: a condition can add an
	// adjective to the target's rendered name, and SendTrio's redaction must
	// see the exact string the line prints.
	casterName, targetName := c.casterName(), c.targetName()
	casterHidden, targetHidden := c.hiddenFromRoom()
	if target := spellConditionTargetOf(c.target); target != nil {
		for _, conditionId := range c.spell.ConditionIds {
			applySpellCondition(target, c.spell, c.casterChar, conditionId)
		}
	}
	if c.spell.IsHarm() {
		commitHarmfulSpellAggro(c, fresh)
	}
	// KNOWN AND DEFERRED: a condition with authored start text also narrates
	// this moment through the event applySpellCondition queues, so an
	// audience can read it twice. The messaging arc's M6 merges them.
	if c.selfCast() {
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(c.category(), fmt.Sprintf(
				`Your %s takes effect.%s`, c.spell.Name, c.critTag())),
			Actee: messaging.NoLine,
			Observer: messaging.Say(c.category(), fmt.Sprintf(
				`<ansi fg="cyan">%s</ansi> settles over %s.`, c.spell.Name, casterName)),
		}, c.selfCastAudience(casterName))
		return 0
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(c.category(), fmt.Sprintf(
			`Your %s takes effect on %s!%s`, c.spell.Name, targetName, c.critTag())),
		Actee: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> takes effect on you!%s`, casterName, c.spell.Name, c.critTag())),
		Observer: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> settles over %s.`,
			roomName(casterHidden, casterName), c.spell.Name, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// applySpellHeal is the one heal applier (slice 3b): a Regenerating record
// on the target at the spell's magnitude as a regen multiplier (floored at
// 1x) for half the universal spell duration (floored at six rounds), read
// from the caster's primarystat and cast skill. Every pairing gains it; a
// mob's heal on a player used to apply nothing (audit row 3). The dead
// player-to-player crit boost is gone (owner ruling 3).
//
// A player healing a mob queues events.Healed, which the AI companion reads
// as "somebody tended me"; the event names a mob, so no other pairing
// queues it.
func applySpellHeal(c spellEffectCtx) int {
	if spellStatusDefended(c) {
		return 0
	}
	stat, skill := spellCasterStatAndSkill(c.spell, c.casterChar)
	regenMult := float64(c.magnitude)
	if regenMult < 1.0 {
		regenMult = 1.0
	}
	durationRounds := calcSpellDuration(c.spell.BaseFolds, skill, stat) / 2
	if durationRounds < 6 {
		durationRounds = 6
	}
	casterName, targetName := c.casterName(), c.targetName()
	casterHidden, targetHidden := c.hiddenFromRoom()
	if u, m := c.casterUser(), c.targetMob(); u != nil && m != nil {
		events.AddToQueue(events.Healed{HealerUserId: u.UserId, MobInstanceId: m.InstanceId})
	}
	_ = c.targetChar().AddConditionMagnitude(conditions.ConditionIdRegenerating, durationRounds, regenMult, "heal spell")
	if c.selfCast() {
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(messaging.CategorySpellVital,
				`<ansi fg="green">A warm glow of healing magic envelops you. Your wounds begin to mend.</ansi>`),
			Actee: messaging.NoLine,
			Observer: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`%s channels restorative magic.`, casterName)),
		}, c.selfCastAudience(casterName))
		return 0
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="green">You weave restorative magic around %s.</ansi>`, targetName)),
		Actee: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="green">%s's %s envelops you in healing energy. Your wounds begin to mend.</ansi>`,
			casterName, c.spell.Name)),
		Observer: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> envelops %s in healing light.`,
			roomName(casterHidden, casterName), c.spell.Name, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// applySpellShield is the one shield applier (slice 3b): a Minor Shield
// record on the target worth a third of the caster's primarystat plus its
// weighted cast skill (at least one), scaled by the spell's magnitude (100
// is 1x), for the full universal spell duration. Every pairing gains it: a
// shield on a charmed pet, or from a creature onto a player, applied
// nothing (audit rows 3 and 15). The dead player-to-player crit bump is
// gone (owner ruling 3).
func applySpellShield(c spellEffectCtx) int {
	if spellStatusDefended(c) {
		return 0
	}
	stat, skill := spellCasterStatAndSkill(c.spell, c.casterChar)
	weightedSkill := int(math.Round(float64(skill) * float64(configs.GetBalanceConfig().SkillWeight)))
	shieldBonus := (stat + weightedSkill) / 3
	if shieldBonus < 1 {
		shieldBonus = 1
	}
	// Scale shield strength by spell magnitude (100 = 1.0x baseline).
	if c.magnitude > 0 {
		shieldBonus = int(math.Round(float64(shieldBonus) * float64(c.magnitude) / 100.0))
		if shieldBonus < 1 {
			shieldBonus = 1
		}
	}
	duration := calcSpellDuration(c.spell.BaseFolds, skill, stat)
	casterName, targetName := c.casterName(), c.targetName()
	_, targetHidden := c.hiddenFromRoom()
	_ = c.targetChar().AddConditionMagnitude(conditions.ConditionIdMinorShield, duration, float64(shieldBonus), "spell")
	if c.selfCast() {
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(c.category(),
				`A shimmering magical barrier forms around you, bolstering your defenses.`),
			Actee: messaging.NoLine,
			Observer: messaging.Say(c.category(), fmt.Sprintf(
				`A shimmering barrier surrounds %s.`, casterName)),
		}, c.selfCastAudience(casterName))
		return 0
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(c.category(), fmt.Sprintf(
			`A shimmering magical barrier forms around %s, bolstering their defenses.`, targetName)),
		Actee: messaging.Say(c.category(),
			`A shimmering magical barrier forms around you, bolstering your defenses.`),
		Observer: messaging.Say(c.category(), fmt.Sprintf(
			`A shimmering barrier surrounds %s.`, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// applySpellPurge is the one purge applier (slice 3b): it cancels every
// poison on the target. A mob target gains it: Cleansing Wave over a
// charmed companion said it took effect and cleansed nothing. The
// Go-hooked Purge Affliction spell is a different path
// (resolvePurgeAffliction, spell_purgeaffliction.go) and never comes here.
func applySpellPurge(c spellEffectCtx) int {
	if spellStatusDefended(c) {
		return 0
	}
	// Names are read BEFORE the cure: cancelling the poison can drop an
	// adjective from the target's rendered name.
	casterName, targetName := c.casterName(), c.targetName()
	casterHidden, targetHidden := c.hiddenFromRoom()
	c.targetChar().CancelConditionsWithFlag(conditions.Poison)
	if c.selfCast() {
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(messaging.CategorySpellVital,
				`<ansi fg="green">You purge the afflictions from your body.</ansi>`),
			Actee: messaging.NoLine,
			Observer: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="cyan">%s</ansi> cleanses %s of afflictions.`, c.spell.Name, casterName)),
		}, c.selfCastAudience(casterName))
		return 0
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="green">Your %s cleanses %s of afflictions.</ansi>`, c.spell.Name, targetName)),
		Actee: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
			`<ansi fg="green">%s's %s purges the toxins from your body.</ansi>`, casterName, c.spell.Name)),
		Observer: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> cleanses %s of afflictions.`,
			roomName(casterHidden, casterName), c.spell.Name, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// applySpellDefaultEffect is the arm for an effect with no applier of its
// own (slice 3b): an unknown or empty effect_type, and charm on anything but
// a mob. It applies nothing and says so to both sides. A harmful spell still
// starts the fight (commitHarmfulSpellAggro), as resolveAgainstPlayer used
// to do for every harmful spell. A spell whose narration a Go hook in
// resolveSpell owns (spellNarratedByGoHook) gets no line here.
func applySpellDefaultEffect(c spellEffectCtx) int {
	fresh := !c.targetChar().IsInCombat()
	if spellStatusDefended(c) {
		if c.spell.IsHarm() {
			commitHarmfulSpellAggro(c, fresh)
		}
		return 0
	}
	if c.spell.IsHarm() {
		commitHarmfulSpellAggro(c, fresh)
	}
	if spellNarratedByGoHook(c.spell.SpellId) {
		return 0
	}
	casterName, targetName := c.casterName(), c.targetName()
	if c.selfCast() {
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(c.category(), fmt.Sprintf(`Your %s takes effect.`, c.spell.Name)),
			Actee:    messaging.NoLine,
			Observer: messaging.NoLine,
		}, c.selfCastAudience(casterName))
		return 0
	}
	messaging.SendTrio(messaging.Trio{
		Actor: messaging.Say(c.category(), fmt.Sprintf(
			`Your %s takes effect on %s.`, c.spell.Name, targetName)),
		Actee: messaging.Say(c.category(), fmt.Sprintf(
			`%s's <ansi fg="cyan">%s</ansi> takes effect on you.`, casterName, c.spell.Name)),
		Observer: messaging.NoLine,
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// spellHelpAreaTargets is the one area-help target filler (slice 3b, audit
// row 13), for player and mob casters alike. It returns the players and the
// mobs an area help spell (a mass heal, a cleansing wave) lands on,
// replacing whatever the cast's initiation step put in the target lists.
//
// A player caster, or a mob charmed by a player, stands on that player's
// side: every player in the room, plus every mob charmed by that player or
// by any member of that player's party (actions.HelpCharmAlly, the rule
// single-target help uses too), so a party
// member's companion is healed and a stranger's pet or an enemy is not. The
// AI companion is charmed to its owner permanently
// (modules/aicompanion/commands.go, Charm(owner.UserId, -1, ...)), so it
// counts. A charmed mob caster also lands on itself.
//
// An uncharmed mob caster helps itself and its packmates: the rule its own
// AI uses to choose whom to heal (behaviortree's cast_best_in_category picks
// most_wounded_packmate and tanking_packmate from mobs.FindPackmatesInRoom).
// No player is its ally.
func spellHelpAreaTargets(caster actions.Actor, room *rooms.Room) (userIds []int, mobIds []int) {
	self := actorMob(caster)
	sideUserId := caster.GetUserId()
	if self != nil {
		sideUserId = self.Character.GetCharmedUserId()
		mobIds = append(mobIds, self.InstanceId)
	}
	if sideUserId == 0 {
		for _, pm := range mobs.FindPackmatesInRoom(self) {
			mobIds = append(mobIds, pm.InstanceId)
		}
		return nil, mobIds
	}
	userIds = room.GetPlayers(rooms.FindAll)
	for _, mId := range room.GetMobs(rooms.FindAll) {
		if self != nil && mId == self.InstanceId {
			continue
		}
		if m := mobs.GetInstance(mId); m != nil && actions.HelpCharmAlly(m, sideUserId) {
			mobIds = append(mobIds, mId)
		}
	}
	return userIds, mobIds
}
