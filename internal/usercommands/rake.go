package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// rakeCategories: every role rides CategoryHitNaturalSharp, the room's
// pre-migration category, the player's own lines included, so they wrap
// and colour like the room's (#449).
var rakeCategories = moveCategories{Actor: messaging.CategoryHitNaturalSharp, Actee: messaging.CategoryHitNaturalSharp, Observer: messaging.CategoryHitNaturalSharp}

func Rake(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "rake",
	})
	if handled {
		return true, nil
	}

	// Delegate core resolution to the shared action.
	res := actions.ExecuteRake(actor)
	if res.Cost.Status == characters.CostRefused {
		refusal.Cost(user, res.Cost)
		return true, nil
	}

	if res.NotClawed {
		user.SendText(messaging.CategoryRefusal, "You have no claws to rake with.")
		return true, nil
	}
	if res.OnCooldown {
		refusal.CooldownAfterCost(user, "rake", res.Cost)
		return true, nil
	}
	if res.NoTarget {
		refusal.NoTargetAfterCost(user, "rake", res.Cost)
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
		sendMoveEvent("rake", "hit", ids, aud, rakeCategories, damageTokens)
	} else if res.MoveResult.Damage > 0 {
		// Defended-partial: every role reads the partial tier (sendPartialMove).
		defence, _ := moveDefenceLines(user, room, res.Target, res.MoveResult.Defence, "claw rake")
		sendPartialMove("rake", "partial", ids, aud, rakeCategories, damageTokens, defence.Shortage)
	} else if defence, defended := moveDefenceLines(user, room, res.Target, res.MoveResult.Defence, "claw rake"); defended {
		// A defence stopped it outright: all three lines come from the triad.
		sendMoveDefenceShortage(messaging.Say(messaging.CategoryHitNaturalSharp, defence.Shortage), aud)
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToAttacker),
			Actee:    acteeDefenceLine(targetChar, room, messaging.CategoryHitNaturalSharp, defence.ToDefender, user.Character.Name),
			Observer: messaging.Say(messaging.CategoryHitNaturalSharp, defence.ToRoom),
		}, aud)
	} else {
		sendMoveEvent("rake", "miss", ids, aud, rakeCategories, nil)
	}

	// U6b Task 11: the counter renders AFTER the move's own outcome.
	actions.DispatchCounterMessages(actor, res.Counter)

	return true, nil
}
