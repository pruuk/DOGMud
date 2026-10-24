package actions

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// NarrateSpellPhase is the one sender of a caster's own spell lines, for
// every caster and phase (messaging M6 slice 4): the cast start, each fold
// round, and the magic line. Landing and disruption lines have their own
// bodies in hooks; guard internal/guards/spell_phase_narration_guard_test.go keeps
// every other site from rendering a phase itself.
//
//   - A player caster reads the phase's *_actor line.
//   - Cast phase of a single-target spell only (R2): the first target, when
//     it is a player other than the caster and the spell authors cast_actee,
//     reads cast_actee instead of the room line, the caster hidden by the
//     target's sight (owner ruling R1).
//   - The room reads the phase's *_observer line, or the generic pool line
//     when none is authored (R7), by sight only, names hidden by each
//     reader's sight. A hidden player target reads "something" to each
//     reader who does not perceive them (#504), as a hidden mob target does.
//   - A hidden caster's lines skip every player who does not perceive them
//     (R3), and at cast start each of those players hears one unnamed
//     cast_started_unseen line instead (R5).
func NarrateSpellPhase(caster Actor, spell *spells.SpellData, phase spells.Phase,
	targetUserIds, targetMobIds []int, room *rooms.Room, cat messaging.Category) {

	if caster == nil || spell == nil || room == nil {
		return
	}
	var casterUser *users.UserRecord
	var casterMob *mobs.Mob
	switch a := caster.(type) {
	case *UserActor:
		if a.User == nil {
			return
		}
		casterUser = a.User
	case *MobActor:
		if a.Mob == nil {
			return
		}
		casterMob = a.Mob
	}
	cc := caster.GetCharacter()
	if cc == nil {
		return
	}
	actorName, actorPlain := cc.GetCharacterName(true), cc.GetCharacterName(false)
	if casterMob != nil {
		actorName = MobSeenName(casterMob, room, 0)
	}
	unseenBy := UnseenBy(room, cc)

	ctx := textutil.TokenContext{ActorName: actorName, ActorPlainName: actorPlain}
	// hideActee is the actee name the lines hide per reader. A hidden mob
	// target already renders as the combat-anon "something", which is no
	// name to hide, so it hides nothing (messaging.NoName).
	hideActee := messaging.NoName
	var target *users.UserRecord
	// targetUnseenBy are the readers who do not perceive a hidden player
	// target (#504): the room line reads "something" in its name for them,
	// as a hidden mob target reads for everyone.
	var targetUnseenBy []int
	if len(targetUserIds) > 0 {
		if tu := users.GetByUserId(targetUserIds[0]); tu != nil {
			ctx.ActeeName, ctx.ActeePlainName = tu.Character.GetCharacterName(true), tu.Character.GetCharacterName(false)
			hideActee = ctx.ActeeName
			targetUnseenBy = UnseenBy(room, tu.Character)
			if casterUser == nil || tu.UserId != casterUser.UserId {
				target = tu
			}
		}
	} else if len(targetMobIds) > 0 {
		if tm := mobs.GetInstance(targetMobIds[0]); tm != nil {
			ctx.ActeeName, ctx.ActeePlainName = MobDisplayName(tm, room, 0), tm.Character.GetCharacterName(false)
			if !MobHiddenFrom(tm, 0) {
				hideActee = ctx.ActeeName
			}
		}
	}

	// Authored roles render together (they are index-paired); the generic
	// room pool renders on its own, since Variants.Len is 0 for pools of
	// different lengths, and through spells.CastRoomLine so the line varies
	// across the pool (textutil.Narrate always picks the first).
	roles := spell.Narrate(phase, ctx)
	observer := roles.Observer
	if observer == "" {
		observer = spells.CastRoomLine(phase, ctx)
	}

	aud := messaging.Audience{ActorName: actorName, ActeeName: hideActee, Room: room}
	// The observer half is sent below, not by SendTrio (see there), so it is
	// a considered silence here.
	trio := messaging.Trio{Actor: messaging.NoLine, Actee: messaging.NoLine, Observer: messaging.NoLine}
	exclude := slices.Clone(unseenBy)
	if casterUser != nil {
		aud.Actor, aud.ActorId = casterUser, casterUser.UserId
		trio.Actor = messaging.Say(cat, roles.Actor)
		exclude = append(exclude, casterUser.UserId)
	}
	if phase == spells.PhaseCast && spell.Targeting == combatvocab.TargetSingle && target != nil &&
		roles.Actee != "" && !slices.Contains(unseenBy, target.UserId) {
		aud.Actee, aud.ActeeId = target, target.UserId
		trio.Actee = messaging.Say(cat, roles.Actee)
		exclude = append(exclude, target.UserId)
	}
	messaging.SendTrio(trio, aud)

	// The room line goes here rather than through SendTrio, whose observer
	// half cannot skip the readers who do not perceive a hidden caster.
	if observer != "" {
		room.SendTextVisualHidingNames(cat, observer, []string{actorName, hideActee}, append(slices.Clone(exclude), targetUnseenBy...)...)
		if len(targetUnseenBy) > 0 {
			// Only the readers who do not perceive the hidden target, with its
			// name read at no sight ("something"); the caster still hidden by
			// each reader's own sight.
			others := slices.Clone(exclude)
			for _, uid := range room.GetPlayers() {
				if !slices.Contains(targetUnseenBy, uid) {
					others = append(others, uid)
				}
			}
			room.SendTextVisualHidingNames(cat, messaging.HideNames(observer, []string{hideActee}, messaging.SightNone),
				[]string{actorName}, others...)
		}
	}
	if phase == spells.PhaseCast {
		for _, uid := range unseenBy {
			if casterUser != nil && uid == casterUser.UserId {
				continue
			}
			if u := users.GetByUserId(uid); u != nil {
				u.SendText(cat, spells.CastStartedUnseenLine())
			}
		}
	}
}
