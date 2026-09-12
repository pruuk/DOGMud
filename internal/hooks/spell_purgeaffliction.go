package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
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

// resolvePurgeAffliction narrates the purge and cancels poison on the target.
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

	target.char.CancelBuffsWithFlag(buffs.Poison)
	target.char.RemoveCondition(characters.ConditionPoisoned)
}
