package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// grappleCategories: every role rides CategoryGrappleFlow, so the player's
// own lines wrap and colour like the room's (#449).
var grappleCategories = moveCategories{
	Actor:    messaging.CategoryGrappleFlow,
	Actee:    messaging.CategoryGrappleFlow,
	Observer: messaging.CategoryGrappleFlow,
}

func Grapple(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "grapple",
	})
	if handled {
		return true, nil
	}

	res := actions.ExecuteGrapple(actor)
	if res.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(res.Cost))
		return true, nil
	}

	if res.Crafting {
		// Safety net — should have been caught by the pre-reject above.
		user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't grapple while focused on your work. Finish or be interrupted first.</ansi>`)
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
	if res.TargetGrappling {
		user.SendText(messaging.CategorySystem, fmt.Sprintf("%s is already grappling.", res.Target.Name))
		return true, nil
	}

	if res.GrappleImmune {
		// ⚠️ FOUR different refusals reach here and they are NOT the same
		// thing. Two are about the player's own body and carry NO Target, so the
		// old single message printed an EMPTY name for them. The other two are
		// opposites: incorporeal (nothing to hold) versus immovable (too solid to
		// shift). The Arena Champion is the immovable case -- colossus-form -- and
		// telling that player their hands passed through it was simply false.
		switch res.ImmuneReason {
		case actions.GrappleImmuneSelfSpecies:
			user.SendText(messaging.CategorySystem,
				"Your body has nothing that can take hold. Grappling is not a thing you can do.")
		case actions.GrappleImmuneSelfNoArms:
			user.SendText(messaging.CategorySystem,
				"Grappling means seizing and holding, and you have no arms to do it with.")
		case actions.GrappleImmuneTargetImmovable:
			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				"You get both hands on %s and heave. It might as well be a boulder. Nothing shifts.",
				res.Target.Name))
		default:
			// Incorporeal, and the safe landing spot for any reason added later.
			// Guarded on the name because a future no-Target reason must not
			// reintroduce the empty-name bug.
			if res.Target.Name == "" {
				user.SendText(messaging.CategorySystem, "You cannot get a grip on that.")
			} else {
				user.SendText(messaging.CategorySystem, fmt.Sprintf(
					"You reach for %s but your hands pass right through!", res.Target.Name))
			}
		}
		return true, nil
	}

	if !res.Executed {
		return true, nil
	}

	target := res.Target
	result := res.MoveResult
	targetName := target.Name
	targetPlayerId := target.UserId

	// Resolve the target user record for direct messaging (player targets).
	var targetChar *users.UserRecord
	if target.UserId > 0 {
		targetChar = users.GetByUserId(target.UserId)
	}

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
		ActeeId:   targetPlayerId,
		ActeeName: targetName,
		Room:      room,
	}

	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, user.Character.Name),
		ActorPlain: user.Character.Name,
		Actee:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, targetName),
		ActeePlain: targetName,
	}

	// Send messages based on result
	if result.Success {
		positionTokens := map[string]string{movenarration.TokenPosition: result.PositionDesc}
		sendMoveEvent("grapple", "player_success", ids, aud, grappleCategories, positionTokens)

		// Flavor text for prone targets.
		//
		// PRIVATE KNOWLEDGE, so NoLine twice, deliberately. This explains the
		// actor's own roll to the actor. A room line here would invent an
		// observation nobody in the room made, about a grapple the event above
		// already narrated to them.
		if result.PositionPenalty < 0 {
			sendMoveEvent("grapple", "player_prone_penalty", ids, aud, grappleCategories, nil)
		}

		// Disarm messaging: a WORLD EVENT, so it carries the full trio, which
		// it already did before the migration.
		if result.DisarmResult != nil {
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(grappleCategories.Actor, result.DisarmResult.Message),
				Actee:    messaging.Say(grappleCategories.Actee, result.DisarmResult.TargetMsg),
				Observer: messaging.Say(messaging.CategoryGrappleFlow, result.DisarmResult.RoomMessage),
			}, aud)
		}
	} else {
		sendMoveEvent("grapple", "player_fail", ids, aud, grappleCategories, nil)

		// Defense penalty: private knowledge again, same ruling as above.
		if result.DefensePenalty {
			sendMoveEvent("grapple", "player_defense_exposed", ids, aud, grappleCategories, nil)
		}

		// Critical failure: a world event, full trio, as before.
		if result.CritFailure != nil {
			messaging.SendTrio(messaging.Trio{
				Actor:    messaging.Say(grappleCategories.Actor, result.CritFailure.Message),
				Actee:    messaging.Say(grappleCategories.Actee, result.CritFailure.TargetMessage),
				Observer: messaging.Say(messaging.CategoryGrappleFlow, result.CritFailure.RoomMessage),
			}, aud)
		}
	}

	return true, nil
}
