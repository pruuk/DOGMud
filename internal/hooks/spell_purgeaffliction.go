package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// purgeTarget is whoever Purge Affliction was aimed at: a player (user set),
// a mob such as a charmed companion (user nil, display set), or the caster.
type purgeTarget struct {
	char    *characters.Character
	user    *users.UserRecord // nil for a mob
	name    string            // plain name, for the seam to hide
	display string            // rendered mob name; empty for a player
}

// token renders the target's name as the lines print it. A mob's rendered name
// already carries its own identity tag and duplicate index, so it is used as
// it comes; a player is wrapped in the username tag here.
func (p purgeTarget) token() string {
	if p.display != "" {
		return p.display
	}
	return fmt.Sprintf(`<ansi fg="username">%s</ansi>`, p.name)
}

// stillPresent mirrors the target loops' own admission (spell_resolution.go
// :137-152 for mobs, :159-170 for players): a target that died or left the room
// while the spell was folding is neither purged nor narrated.
//
// Purge Affliction is waitrounds 2, so a companion wandering off or dropping
// mid-fold is ordinary, not exotic. Charm carried the identical defect for the
// identical reason and was fixed the same way: a raw read of the target id
// AFTER the loop ignores every filter the loop applies. A failing check must
// fall through to nothing, never to the self-cast arm, which would purge the
// caster all over again.
//
// requireAlive is true for a mob target (the mob loop skips a dead mob) and
// false for a player target: the player loop only skips a downed target for
// harm spells, so a downed ally still present in the room is purged, as it
// was before this admission existed.
func (p purgeTarget) stillPresent(room *rooms.Room, requireAlive bool) bool {
	if p.char == nil || room == nil || p.char.RoomId != room.RoomId {
		return false
	}
	return !requireAlive || p.char.Health > 0
}

// purgeAfflictions ends what Purge Affliction and Cleansing Wave cure: every
// poison, and every record a damage-over-time spell lands (Blood Boil's
// Boiling Blood, #249). The cure follows the spells, read from the spell
// registry through spellDotConditionId, so a new dot spell's record is
// curable the day it ships, and a combat bleed (122), which no spell lands,
// stays uncured as before (owner call 2026-10-10).
func purgeAfflictions(ch *characters.Character) {
	ch.CancelConditionsWithFlag(conditions.Poison)
	for _, s := range spells.GetAllSpells() {
		if s.EffectType != "dot" {
			continue
		}
		// GetConditions skips a record already expired, by the poison
		// cancel above (121) or an earlier dot spell naming the same one.
		if id := spellDotConditionId(s); len(ch.GetConditions(id)) > 0 {
			ch.RemoveCondition(id)
		}
	}
}

// resolvePurgeAffliction narrates the purge and cures the target (purgeAfflictions).
// Self-cast keeps its one-line wording. A mob target has no client, so its
// Actee recipient is nil and only the caster and the room read anything; names
// are hidden per reader by the seam.
func resolvePurgeAffliction(user *users.UserRecord, room *rooms.Room, target purgeTarget) {
	if user == nil || target.char == nil {
		mudlog.Error("resolvePurgeAffliction", "error", "nil user or target")
		return
	}

	if target.char == user.Character {
		user.SendText(messaging.CategorySpellVital, `<ansi fg="green">You purge the afflictions from your body.</ansi>`)
		if room != nil {
			sendVisualRoomText(room, messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="username">%s</ansi> purges their afflictions.`,
				user.Character.Name), user.UserId)
		}
	} else {
		// ⚠️ The Actee recipient is declared as the interface and left unset
		// for a mob: a typed nil *users.UserRecord would make it non-nil and
		// SendTrio would call through it.
		var actee messaging.Recipient
		acteeId := 0
		if target.user != nil {
			actee = target.user
			acteeId = target.user.UserId
		}
		aud := messaging.Audience{
			Actor: user, ActorId: user.UserId, ActorName: user.Character.Name,
			Actee: actee, ActeeId: acteeId, ActeeName: target.name,
		}
		if room != nil {
			aud.Room = room
		}
		messaging.SendTrio(messaging.Trio{
			Actor: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green">You direct purging energy towards %s.</ansi>`, target.token())),
			Actee: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="green"><ansi fg="username">%s</ansi> purges the afflictions from your body.</ansi>`,
				user.Character.Name)),
			Observer: messaging.Say(messaging.CategorySpellVital, fmt.Sprintf(
				`<ansi fg="username">%s</ansi> directs purging energy towards %s.`,
				user.Character.Name, target.token())),
		}, aud)
	}

	purgeAfflictions(target.char)
}
