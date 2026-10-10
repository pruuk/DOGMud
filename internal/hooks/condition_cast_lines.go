package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Messaging M6 slice 1, section 2: a condition's start is one line per
// audience, written by the condition's data and naming its caster. The
// caster reads start_actor, the holder start_actee, the room start_observer.
// These helpers are ApplyConditions' half of that rule; the spell's half
// (dropping its generic trio) is applySpellConditionEffect.

// critMarker is what a spell's caster line carries on a critical cast
// (spellEffectCtx.critTag). A condition's caster line carries it too, once it
// replaces the spell's own line.
const critMarker = ` <ansi fg="yellow">[CRIT!]</ansi>`

// conditionParty is one side of a condition's start lines: its tagged and
// plain names as a reader who perceives it reads them, its room, its client
// when it is an online player, and the mob when it is one.
type conditionParty struct {
	name, plain string
	roomId      int
	user        *users.UserRecord
	mob         *mobs.Mob
}

// namesFor is the party's names as one reader reads them (viewer 0 is the
// room as a whole). A hidden mob reads "something" to a reader who does not
// perceive it and its own name to one who does (mobDisplayName's viewer
// rule, #382), as a spell's own lines read.
func (p conditionParty) namesFor(viewerUserId int) (name, plain string) {
	if p.mob == nil {
		return p.name, p.plain
	}
	r := rooms.LoadRoom(p.mob.Character.RoomId)
	if r == nil {
		return p.name, p.plain
	}
	if mobHiddenFrom(p.mob, viewerUserId) {
		return messaging.StripNameAdjectives(mobDisplayName(p.mob, r, viewerUserId)), messaging.UnseenNoun(messaging.SightNone)
	}
	return messaging.StripNameAdjectives(mobSeenName(p.mob, r, viewerUserId)), p.mob.Character.GetCharacterName(false)
}

// conditionPartyOf resolves a holder or a caster by ref. A player's names are
// GetCharacterName's; a mob's are conditionMobNames', which name a hidden mob
// too: a holder's room line reaches only the readers who perceive it (#458),
// and namesFor hides a mob from any other reader. ok is false for nobody, an
// offline player or a gone mob.
func conditionPartyOf(ref state.ActorRef) (conditionParty, bool) {
	if ref.UserId != 0 {
		u := users.GetByUserId(ref.UserId)
		if u == nil {
			return conditionParty{}, false
		}
		return conditionParty{
			name:   u.Character.GetCharacterName(true),
			plain:  u.Character.GetCharacterName(false),
			roomId: u.Character.RoomId,
			user:   u,
		}, true
	}
	if ref.MobInstanceId != 0 {
		m := mobs.GetInstance(ref.MobInstanceId)
		if m == nil {
			return conditionParty{}, false
		}
		name, plain := conditionMobNames(m)
		return conditionParty{name: name, plain: plain, roomId: m.Character.RoomId, mob: m}, true
	}
	return conditionParty{}, false
}

// conditionSelfCast reports whether the condition's caster is its holder.
func conditionSelfCast(evt events.Condition) bool {
	c := evt.Caster
	return (c.UserId != 0 && c.UserId == evt.UserId) ||
		(c.MobInstanceId != 0 && c.MobInstanceId == evt.MobInstanceId)
}

// conditionReaderSight is how well a participant sees the other party of a
// start line: the decision the pre-landing snapshot recorded for them when
// there is one (a darkness source's start, owner rule 2026-10-05), else the
// room as it is now.
func conditionReaderSight(r *rooms.Room, snap rooms.VisualSnapshot, userId int) messaging.SightDecision {
	if d, ok := snap[userId]; ok {
		return d
	}
	return r.ParticipantSight(userId)
}

// narrateConditionStart sends a landing condition's start lines: start_actor
// to a caster who is someone else and online, start_actee to a player
// holder, start_observer to the room (unless hideOnHidden). {actor} is the
// caster, "something" when nobody cast it. Each private line hides the other
// party's name from a reader who cannot make them out, as SendTrio does, and
// the room line hides both, excludes both, and skips unseenBy, the readers
// who did not perceive the holder before it landed (conditionLineUnseenBy,
// #458). The crit marker rides the caster's line, which on a self-cast is
// the holder's.
func narrateConditionStart(spec *conditions.ConditionSpec, evt events.Condition, holder conditionParty,
	snap rooms.VisualSnapshot, hideOnHidden bool, unseenBy []int) {

	selfCast := conditionSelfCast(evt)
	caster, casterKnown := conditionParty{}, false
	if !selfCast {
		caster, casterKnown = conditionPartyOf(evt.Caster)
	}
	// {actor} for a reader: the caster, the holder on a self-cast, and
	// "something" when nobody cast it.
	casterNamesFor := func(viewerUserId int) (string, string) {
		switch {
		case selfCast:
			return holder.name, holder.plain
		case casterKnown:
			return caster.namesFor(viewerUserId)
		}
		unseen := messaging.UnseenNoun(messaging.SightNone)
		return unseen, unseen
	}
	crit := ""
	if evt.CasterCrit {
		crit = critMarker
	}
	r := rooms.LoadRoom(holder.roomId)

	// The holder is the ACTEE: the condition happens to them. A mob holder
	// has no client, so it reads nothing.
	if holder.user != nil {
		casterName, casterPlain := casterNamesFor(holder.user.UserId)
		line := spec.NarrateCast(conditions.PhaseStart, holder.name, holder.plain, casterName, casterPlain).Actee
		if line != "" {
			if selfCast {
				line += crit
			}
			if casterKnown && r != nil {
				line = messaging.HideNames(line, []string{casterPlain}, conditionReaderSight(r, snap, holder.user.UserId))
			}
			holder.user.SendText(messaging.CategoryConditionApply, line)
		}
	}

	if casterKnown && caster.user != nil {
		holderName, holderPlain := holder.namesFor(caster.user.UserId)
		line := spec.NarrateCast(conditions.PhaseStart, holderName, holderPlain, caster.name, caster.plain).Actor
		if line != "" {
			line += crit
			if r != nil {
				line = messaging.HideNames(line, []string{holderPlain}, conditionReaderSight(r, snap, caster.user.UserId))
			}
			caster.user.SendText(messaging.CategoryConditionApply, line)
		}
	}

	casterName, casterPlain := casterNamesFor(0)
	roles := spec.NarrateCast(conditions.PhaseStart, holder.name, holder.plain, casterName, casterPlain)
	// Visual, not audio: start text describes what the room SEES. HidingNames,
	// because a start line may author a bare {actee_plain} or {actor_plain},
	// which tag-based Anonymize cannot see.
	if roles.Observer != "" && !hideOnHidden && r != nil {
		names := []string{holder.plain}
		exclude := append([]int{}, unseenBy...)
		if holder.user != nil {
			exclude = append(exclude, holder.user.UserId)
		}
		if casterKnown {
			names = append(names, caster.plain)
			if caster.user != nil {
				exclude = append(exclude, caster.user.UserId)
			}
		}
		sendConditionStartRoomText(r, snap, roles.Observer, names, exclude...)
	}
}

// familyRivalNames names the held records a landing condition will replace
// (conditions.Conditions.FamilyRivals), read BEFORE the add discards them.
func familyRivalNames(held *conditions.Conditions, conditionId int) []string {
	var names []string
	for _, b := range held.FamilyRivals(conditionId) {
		if spec := conditions.GetConditionSpec(b.ConditionId); spec != nil {
			names = append(names, spec.Name)
		}
	}
	return names
}

// narrateFamilyReplacement tells the holder and the room which ward or heal
// gave way to the one landing (owner ruling R6), in place of the old record's
// end line, which the discard never tells. The room line skips unseenBy, as
// every condition room line does (#458).
func narrateFamilyReplacement(spec *conditions.ConditionSpec, replaced []string, holder conditionParty, unseenBy []int) {
	r := rooms.LoadRoom(holder.roomId)
	for _, old := range replaced {
		if holder.user != nil {
			holder.user.SendText(messaging.CategoryConditionApply,
				fmt.Sprintf(`Your %s fades as %s takes hold.`, old, spec.Name))
		}
		if r == nil {
			continue
		}
		exclude := append([]int{}, unseenBy...)
		if holder.user != nil {
			exclude = append(exclude, holder.user.UserId)
		}
		r.SendTextVisualHidingNames(messaging.CategoryConditionApply,
			fmt.Sprintf(`%s's %s fades as %s takes hold.`, holder.name, old, spec.Name),
			[]string{holder.plain}, exclude...)
	}
}
