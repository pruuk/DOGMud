package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// maulCategories: every role rides CategoryHitNaturalSharp, the room's
// pre-migration category, the player's own lines included, so they wrap
// and colour like the room's (#449).
var maulCategories = moveCategories{Actor: messaging.CategoryHitNaturalSharp, Actee: messaging.CategoryHitNaturalSharp, Observer: messaging.CategoryHitNaturalSharp}

func Maul(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "maul",
	})
	if handled {
		return true, nil
	}

	// Delegate core resolution to the shared action.
	res := actions.ExecuteMaul(actor)
	if res.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(res.Cost))
		return true, nil
	}

	if res.NotFanged {
		user.SendText(messaging.CategorySystem, "You have no fangs to maul with.")
		return true, nil
	}
	if res.OnCooldown {
		user.SendText(messaging.CategorySystem, "You need a moment to recover before attempting another special move.")
		return true, nil
	}
	if res.NoTarget {
		user.SendText(messaging.CategorySystem, "You have no target!")
		return true, nil
	}

	targetName := res.Target.Name

	// Resolve player target for direct messaging.
	var targetChar *users.UserRecord
	if res.Target.UserId > 0 {
		targetChar = users.GetByUserId(res.Target.UserId)
	}

	dmgDesc := combat.GetDamageDescription(res.MoveResult.Damage, res.MoveResult.TargetMaxHP)

	// Declared as the interface and left unset for a mob target. Assigning a
	// typed-nil *users.UserRecord would make it a non-nil interface value.
	var acteeRecipient messaging.Recipient
	if targetChar != nil {
		acteeRecipient = targetChar
	}
	aud := messaging.Audience{
		Actor:     user,
		ActorId:   user.UserId,
		ActorName: user.Character.Name,
		Actee:     acteeRecipient,
		ActeeId:   res.Target.UserId,
		ActeeName: targetName,
		Room:      room,
	}

	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, user.Character.Name),
		ActorPlain: user.Character.Name,
		Actee:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, targetName),
		ActeePlain: targetName,
	}
	damageTokens := map[string]string{movenarration.TokenDamage: dmgDesc}

	if res.MoveResult.Hit {
		sendMoveEvent("maul", "player_hit", ids, aud, maulCategories, damageTokens)
	} else if res.MoveResult.Damage > 0 {
		// Defended-partial: the personal lines carry the damage, and the room
		// line names the defence that blunted the maul (U6b Task 9), falling
		// back to the squared partial text when there was no defence to name.
		roles, _ := renderMoveEvent("maul", "player_partial", ids, damageTokens)
		defence, defended := moveDefenceLines(user, room, res.Target, res.MoveResult.Defence, "savage bite")
		observer := lineOrNone(messaging.CategoryHitNaturalSharp, roles.Observer)
		if defended {
			observer = messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom)
			sendMoveDefenceShortage(targetChar, defence)
		}
		messaging.SendTrio(messaging.Trio{
			Actor:    lineOrNone(maulCategories.Actor, roles.Actor),
			Actee:    lineOrNone(maulCategories.Actee, roles.Actee),
			Observer: observer,
		}, aud)
	} else if defence, defended := moveDefenceLines(user, room, res.Target, res.MoveResult.Defence, "savage bite"); defended {
		// A defence stopped it outright: all three lines come from the triad.
		sendMoveDefenceShortage(targetChar, defence)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToAttacker),
			Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryHitNaturalSharp, defence.ToDefender, user.Character.Name),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom),
		}, aud)
	} else {
		sendMoveEvent("maul", "player_miss", ids, aud, maulCategories, nil)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(actor, res.Counter)

	return true, nil
}
