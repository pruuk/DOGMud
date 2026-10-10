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
	"github.com/GoMudEngine/GoMud/internal/spells"
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
	// Read before the conditions land, as the apply hook's refresh test is:
	// a condition already held narrates no start.
	narrated := spellConditionsNarrateStart(c)
	if target := spellConditionTargetOf(c.target); target != nil {
		for _, conditionId := range c.spell.ConditionIds {
			applySpellCondition(target, c.spell, c.casterChar, conditionId, c.casterRef(), narrated, narrated && c.out.AttackerCrit)
		}
	}
	if c.spell.IsHarm() {
		commitHarmfulSpellAggro(c, fresh)
	}
	// Owner ruling R11: when the conditions' own start lines tell every
	// audience, they are the only lines (narrateConditionStart) and the
	// generic trio below is not sent. A silent condition, or a re-cast of one
	// already held, keeps the trio.
	if narrated {
		return 0
	}
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

// spellConditionsNarrateStart reports whether every condition the spell lands
// will tell its own start to every audience (ConditionSpec.NarratesCastStart),
// so the spell sends no generic "takes effect" lines of its own (owner ruling
// R11). A condition the target already holds is a refresh with no start
// lines, so it keeps the spell's lines; so does a spell that lands nothing.
func spellConditionsNarrateStart(c spellEffectCtx) bool {
	if len(c.spell.ConditionIds) == 0 {
		return false
	}
	for _, conditionId := range c.spell.ConditionIds {
		if !spellConditionNarratesStart(c, conditionId) {
			return false
		}
	}
	return true
}

// spellConditionNarratesStart is spellConditionsNarrateStart for one
// condition: a fresh landing whose own start lines tell every audience.
func spellConditionNarratesStart(c spellEffectCtx, conditionId int) bool {
	spec := conditions.GetConditionSpec(conditionId)
	return spec != nil && !c.targetChar().HasCondition(conditionId) && spec.NarratesCastStart(c.selfCast())
}

// applySpellHeal is the one heal applier (slice 3b): the spell's own heal
// (spellHealConditionId) on the target at the spell's magnitude as a regen
// multiplier (floored at 1x), for the heal's own duration scaled by the
// caster (healTriggers). Each heal spell has its own multiplier and duration
// (messaging M6 slice 1, owner rulings R8 and R9). The heal travels the
// condition event, which replaces any other heal the target holds and names
// the caster; when the heal's own start lines tell every audience, they are
// the only lines (owner ruling R11), and a re-cast of a heal already held
// keeps the lines below. Every pairing gains it; a mob's heal on a player
// used to apply nothing (audit row 3). The dead player-to-player crit boost
// is gone (owner ruling 3).
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
	healId := spellHealConditionId(c.spell)
	casterName, targetName := c.casterName(), c.targetName()
	casterHidden, targetHidden := c.hiddenFromRoom()
	if u, m := c.casterUser(), c.targetMob(); u != nil && m != nil {
		events.AddToQueue(events.Healed{HealerUserId: u.UserId, MobInstanceId: m.InstanceId})
	}
	narrated := spellConditionNarratesStart(c, healId)
	if target := spellConditionTargetOf(c.target); target != nil {
		target.QueueCondition(events.Condition{ConditionId: healId, Source: "heal spell",
			Triggers: healTriggers(healId, stat, skill), Magnitude: regenMult, Caster: c.casterRef(), CastStart: narrated})
	}
	if narrated {
		return 0
	}
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

// applySpellShield is the one shield applier (slice 3b): the spell's own
// ward (spellWardConditionId) on the target, worth a third of the caster's
// primarystat plus its weighted cast skill (at least one), scaled by the
// spell's magnitude (100 is 1x), for the full universal spell duration. That
// one strength feeds every kind of damage the ward blocks (messaging M6
// slice 1, owner ruling R7). The ward travels the condition event, which
// replaces any other ward the target holds and names the caster; when the
// ward's own start lines tell every audience, they are the only lines (owner
// ruling R11), and a re-cast of a ward already held keeps the lines below.
// Every pairing gains it: a shield on a charmed pet, or from a creature onto
// a player, applied nothing (audit rows 3 and 15). The dead player-to-player
// crit bump is gone (owner ruling 3).
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
	wardId := spellWardConditionId(c.spell)
	narrated := spellConditionNarratesStart(c, wardId)
	if target := spellConditionTargetOf(c.target); target != nil {
		target.QueueCondition(events.Condition{ConditionId: wardId, Source: "spell",
			Triggers: duration, Magnitude: float64(shieldBonus), Caster: c.casterRef(), CastStart: narrated})
	}
	if narrated {
		return 0
	}
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

// spellHealConditionId is the heal a heal spell lands: its one
// condition_ids entry, a heal-family record (the root guard
// TestShieldAndHealSpellsLandTheirOwnCondition holds the shipped spells to
// that), or Regenerating for a spell that names none.
func spellHealConditionId(spell *spells.SpellData) int {
	if len(spell.ConditionIds) > 0 {
		return spell.ConditionIds[0]
	}
	return conditions.ConditionIdRegenerating
}

// spellDurationTerm is the caster's part of every universal spell duration
// (calcSpellDuration): 10 + willpower/20 + spellcasting skill/2.
func spellDurationTerm(spellcastingSkill int, willpower int) float64 {
	return 10.0 + float64(willpower)/20.0 + float64(spellcastingSkill)/2.0
}

// untrainedDurationTerm is spellDurationTerm for an untrained caster at the
// stat centre of 100: the caster a heal's authored duration is written for.
var untrainedDurationTerm = spellDurationTerm(0, 100)

// healTriggers is how many rounds a heal lasts from this caster: the heal
// condition's authored trigger count, which is its duration for an untrained
// caster of average willpower, scaled by how much longer this caster's spells
// last (spellDurationTerm), at least one. A heal's identity, long or short,
// is its own data, so each heal can be gentle and long or strong and short
// whatever its casting time (owner ruling R9); the caster still lengthens
// every heal as before.
func healTriggers(conditionId int, willpower int, spellcastingSkill int) int {
	spec := conditions.GetConditionSpec(conditionId)
	if spec == nil {
		return 1
	}
	return max(1, int(math.Round(float64(spec.TriggerCount)*spellDurationTerm(spellcastingSkill, willpower)/untrainedDurationTerm)))
}

// spellWardConditionId is the ward a shield spell lands: its one
// condition_ids entry, a ward-family record (the root guard
// TestShieldAndHealSpellsLandTheirOwnCondition holds the shipped spells to
// that), or Conviction Ward for a spell that names none.
func spellWardConditionId(spell *spells.SpellData) int {
	if len(spell.ConditionIds) > 0 {
		return spell.ConditionIds[0]
	}
	return conditions.ConditionIdConvictionWard
}

// applySpellPurge is the one purge applier (slice 3b): it cures the target
// (purgeAfflictions: every poison and every record a dot spell lands). A mob target gains it: Cleansing Wave over a
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
	purgeAfflictions(c.targetChar())
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
